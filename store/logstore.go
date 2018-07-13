package store

import (
	dbm "github.com/tendermint/tmlibs/db"
	"log"
	"os"
)

type LogParams struct {
	LogFilename    string
	LogFlags       int // log.Ldate | log.Ltime | log.LUTC
	LogVersion     bool
	LogDelete      bool
	LogSetKey      bool
	LogSetValue    bool
	LogGet         bool
	LogHas         bool
	LogSaveVersion bool
	LogHash        bool
}

type LogStore struct {
	store  *IAVLStore
	logger log.Logger
	params LogParams
}

func NewLogStore(db dbm.DB) (ls *LogStore, err error) {
	ls = new(LogStore)
	ls.store, err = NewIAVLStore(db)
	ls.params = LogParams{
		LogFilename:    "app-store.log",
		LogFlags:       0,
		LogVersion:     false,
		LogDelete:      true,
		LogSetKey:      true,
		LogSetValue:    false,
		LogGet:         false,
		LogHas:         false,
		LogSaveVersion: false,
		LogHash:        false,
	}

	if err != nil {
		return nil, err
	}
	file, err := os.Create(ls.params.LogFilename)
	if err != nil {
		return nil, err
	}
	ls.logger = *log.New(file, "", ls.params.LogFlags)
	ls.logger.Println("Created new app log store")
	return ls, nil
}

func (s *LogStore) Delete(key []byte) {
	if s.params.LogDelete {
		s.logger.Println("Delete key: ", string(key))
	}
	s.store.Delete(key)
}

func (s *LogStore) Set(key, val []byte) {
	if s.params.LogSetKey {
		s.logger.Println("Set key: ", string(key))
	}
	if s.params.LogSetValue {
		s.logger.Println("Set Value: ", string(val))
	}
	s.store.Set(key, val)
}

func (s *LogStore) Has(key []byte) bool {
	if s.params.LogHas {
		s.logger.Println("Has key: ", string(key))
	}
	return s.store.Has(key)
}

func (s *LogStore) Get(key []byte) []byte {
	val := s.store.Get(key)
	if s.params.LogGet {
		s.logger.Println("Get key: ", string(key), " val: ", val)
	}
	return val
}

func (s *LogStore) Hash() []byte {
	hash := s.store.Hash()
	if s.params.LogHash {
		s.logger.Println("Hash ", hash)
	}
	return hash
}

func (s *LogStore) Version() int64 {
	version := s.store.Version()
	if s.params.LogVersion {
		s.logger.Println("Version ", version)
	}
	return version
}

func (s *LogStore) SaveVersion() ([]byte, int64, error) {
	vByte, vInt, err := s.store.SaveVersion()
	if s.params.LogSaveVersion {
		s.logger.Println("SaveVersion", string(vByte), " int ", vInt, " err ", err)
	}
	return vByte, vInt, err
}
