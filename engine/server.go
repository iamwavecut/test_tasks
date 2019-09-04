package engine

import (
	"bytes"
	"compress/zlib"
	"context"
	"encoding/gob"
	"io/ioutil"
	"log"
	"sync"
	"time"

	"github.com/pkg/errors"
)

type Server struct {
	mx  sync.RWMutex
	wg  sync.WaitGroup
	cfg Config
	// TODO: Add bucketing by type? / hash
	Items   map[string]Item
	isDirty bool
}

type Item struct {
	Payload   []byte
	ExpiresIn time.Duration
	ExpiresAt time.Time
}

func New(ctx context.Context, cfg Config) *Server {
	s := &Server{
		Items: make(map[string]Item),
		cfg:   cfg,
	}

	if cfg.TTLCheckIntervalSecs == 0 {
		cfg.TTLCheckIntervalSecs = 1
	}
	s.debug("TTL check interval secs", cfg.TTLCheckIntervalSecs)

	go func(ctx context.Context) {
		s.wg.Add(1)
		t := time.NewTicker(time.Second * time.Duration(cfg.TTLCheckIntervalSecs))
		defer t.Stop()

		for {
			select {

			case <-ctx.Done():
				s.debug("TTL cancelled")
				s.wg.Done()
				return

			case now := <-t.C:
				s.debug("TTL tick", cfg.TTLCheckIntervalSecs)

				s.mx.Lock()
				for key, item := range s.Items {
					if item.ExpiresAt.Equal(now) || item.ExpiresAt.Before(now) {
						s.debug("TTL expired key, deleting", key)

						delete(s.Items, key)
						s.isDirty = true
					}
				}
				s.mx.Unlock()
			}
		}
	}(ctx)

	if cfg.Storage != nil {
		if err := s.restore(); err != nil {
			log.Println(err)
			s.Items = make(map[string]Item)
		}

		if cfg.StorageDeferredWriteIntervalSecs > 0 {
			s.debug("FLUSH check interval secs", cfg.StorageDeferredWriteIntervalSecs)

			go func(ctx context.Context) {
				s.wg.Add(1)
				t := time.NewTicker(time.Second * time.Duration(cfg.StorageDeferredWriteIntervalSecs))
				defer t.Stop()

				for {
					select {

					case <-ctx.Done():
						s.debug("FLUSH cancelled")
						s.wg.Done()
						return

					case <-t.C:
						s.debug("FLUSH tick", cfg.StorageDeferredWriteIntervalSecs)

						if s.isDirty {
							s.mx.Lock()

							check(s.flush())

							s.mx.Unlock()

							s.isDirty = false
						}
					}
				}
			}(ctx)
		}
	}

	gob.Register(time.Time{})

	go func(ctx context.Context) {
		s.wg.Add(1)
		select {
		case <-ctx.Done():
			if s.isDirty {
				s.debug("state is dirty, flushing")

				if err := s.flush(); err != nil {
					check(err)
				}
				s.isDirty = false
			}
			s.wg.Done()
			return
		}
	}(ctx)

	return s
}

// Adds or updates Item with TTL
func (s *Server) Set(key string, value interface{}, expiresIn time.Duration) error {
	s.debug("setting", key)

	s.mx.Lock()
	defer s.mx.Unlock()

	valueBytes, err := encode(value)
	check(err)
	s.Items[key] = Item{Payload: valueBytes, ExpiresAt: time.Now().Add(expiresIn), ExpiresIn: expiresIn}
	s.isDirty = true

	return nil
}

// Adds Item with TTL
func (s *Server) Add(key string, value interface{}, expiresIn time.Duration) error {
	s.debug("adding", key, value)

	s.mx.Lock()
	defer s.mx.Unlock()

	if _, ok := s.Items[key]; ok {
		return errors.New("already exists")
	}

	valueBytes, err := encode(value)
	check(err)
	s.Items[key] = Item{Payload: valueBytes, ExpiresAt: time.Now().Add(expiresIn), ExpiresIn: expiresIn}
	s.isDirty = true

	return nil
}

// Updates Item value and expiration stamp
func (s *Server) Update(key string, value interface{}) error {
	s.debug("updating", key)

	s.mx.Lock()
	defer s.mx.Unlock()

	existing, ok := s.Items[key]
	if !ok {
		return errors.New("can not update, not exists")
	}
	valueBytes, err := encode(value)
	check(err)
	existing.Payload = valueBytes
	existing.ExpiresAt = time.Now().Add(existing.ExpiresIn)
	s.Items[key] = existing
	s.isDirty = true

	return nil
}

// Removes Item
func (s *Server) Remove(key string) error {
	s.debug("removing", key)

	s.mx.Lock()
	defer s.mx.Unlock()

	if _, ok := s.Items[key]; !ok {
		return errors.New("can not remove, not exists")
	}
	delete(s.Items, key)
	s.isDirty = true

	return nil
}

// Gets Item
func (s *Server) Get(key string, target interface{}) error {
	s.debug("getting", key)

	if target == nil {
		return errors.New("no target specified")
	}

	s.mx.RLock()
	defer s.mx.RUnlock()

	item, ok := s.Items[key]
	if !ok {
		return errors.New("can not get, not exists")
	}
	return decode(item.Payload, &target)
}

// Returns the list of all actual keys
func (s *Server) Keys() (keys []string) {
	s.debug("keys")

	s.mx.RLock()
	defer s.mx.RUnlock()

	for key := range s.Items {
		keys = append(keys, key)
	}

	return keys
}

func (s *Server) restore() error {
	z, err := zlib.NewReader(s.cfg.Storage)
	if err != nil {
		return err
	}
	defer z.Close()

	data, err := ioutil.ReadAll(z)
	if err != nil {
		return err
	}

	if err = gob.NewDecoder(bytes.NewReader(data)).Decode(&s.Items); err != nil {
		return err
	}
	return nil
}
func (s *Server) flush() error {
	z, err := zlib.NewWriterLevel(s.cfg.Storage, zlib.BestCompression)
	if err != nil {
		return err
	}
	defer z.Close()

	if err := gob.NewEncoder(z).Encode(s.Items); err != nil {
		return err
	}
	return nil
}

func (s *Server) Reset() {
	s.mx.Lock()
	s.Items = make(map[string]Item)
	s.mx.Unlock()
	s.isDirty = true
}

func (s *Server) debug(v ...interface{}) {
	if s.cfg.Debug {
		log.Println(v...)
	}
}

func check(err error) {
	if err != nil {
		log.Fatalln("Fatal on err", err)
	}
}

func encode(value interface{}) ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(value)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func decode(encoded []byte, target interface{}) error {
	buf := bytes.NewBuffer(encoded)
	dec := gob.NewDecoder(buf)

	if err := dec.Decode(&target); err != nil {
		return err
	}

	return nil
}
