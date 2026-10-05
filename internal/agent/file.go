package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"hash"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/ozodmeofficial/smarteye/internal/protocol"
)

// fileRx tracks one in-progress incoming file.
type fileRx struct {
	f    *os.File
	path string
	hash hash.Hash
}

var (
	fileMu  sync.Mutex
	fileRxs = map[string]*fileRx{}
)

// downloadsDir is where files pushed from the server are saved on the client.
func downloadsDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.TempDir()
	}
	dir := filepath.Join(home, "Downloads", "SmartEYE")
	_ = os.MkdirAll(dir, 0o755)
	return dir
}

// handleFile implements the receiving side of the transfer protocol.
func (a *Agent) handleFile(env *protocol.Envelope) {
	switch env.Type {
	case protocol.TypeFileOffer:
		var o protocol.FileOffer
		env.Decode(&o)
		// Accept into the SmartEYE downloads folder. Only the paired server can
		// reach this agent, so transfers are auto-accepted and clearly located.
		path := filepath.Join(downloadsDir(), filepath.Base(o.Name))
		f, err := os.Create(path)
		if err != nil {
			log.Printf("agent: file create: %v", err)
			_ = a.conn.SendTyped(protocol.TypeFileAccept, protocol.FileAccept{TransferID: o.TransferID, Accept: false})
			return
		}
		fileMu.Lock()
		fileRxs[o.TransferID] = &fileRx{f: f, path: path, hash: sha256.New()}
		fileMu.Unlock()
		_ = a.conn.SendTyped(protocol.TypeFileAccept, protocol.FileAccept{TransferID: o.TransferID, Accept: true})

	case protocol.TypeFileChunk:
		var c protocol.FileChunk
		env.Decode(&c)
		fileMu.Lock()
		rx := fileRxs[c.TransferID]
		fileMu.Unlock()
		if rx == nil {
			return
		}
		data, err := decodeB64(c.Data)
		if err != nil {
			return
		}
		rx.f.Write(data)
		rx.hash.Write(data)

	case protocol.TypeFileDone:
		var d protocol.FileDone
		env.Decode(&d)
		fileMu.Lock()
		rx := fileRxs[d.TransferID]
		delete(fileRxs, d.TransferID)
		fileMu.Unlock()
		if rx == nil {
			return
		}
		rx.f.Close()
		sum := hex.EncodeToString(rx.hash.Sum(nil))
		if d.Checksum != "" && d.Checksum != sum {
			log.Printf("agent: file checksum mismatch for %s", rx.path)
			_ = os.Remove(rx.path)
			return
		}
		log.Printf("agent: received file %s", rx.path)
	}
}
