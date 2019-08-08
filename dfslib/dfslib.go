/*

This package specifies the application's interface to the distributed
file system (DFS) system to be used in assignment 2 of UBC CS 416
2017W2.

*/

package dfslib

import (
	"fmt"
	"log"
	"net"
	"net/rpc"
	"os"
	"regexp"
	"runtime"
	"strconv"
	"time"
)

// A Chunk is the unit of reading/writing in DFS.
type Chunk [32]byte

// Represents a type of file access.
type FileMode int

const (
	// Read mode.
	READ FileMode = iota

	// Read/Write mode.
	WRITE

	// Disconnected read mode.
	DREAD
)

////////////////////////////////////////////////////////////////////////////////////////////
// <ERROR DEFINITIONS>

// These type definitions allow the application to explicitly check
// for the kind of error that occurred. Each API call below lists the
// errors that it is allowed to raise.
//
// Also see:
// https://blog.golang.org/error-handling-and-go
// https://blog.golang.org/errors-are-values

// Contains serverAddr
type DisconnectedError string

func (e DisconnectedError) Error() string {
	return fmt.Sprintf("DFS: Not connnected to server [%s]", string(e))
}

// Contains chunkNum that is unavailable
type ChunkUnavailableError uint8

func (e ChunkUnavailableError) Error() string {
	return fmt.Sprintf("DFS: Latest verson of chunk [%d] unavailable", e)
}

// Contains filename
type OpenWriteConflictError string

func (e OpenWriteConflictError) Error() string {
	return fmt.Sprintf("DFS: Filename [%s] is opened for writing by another client", string(e))
}

// Contains file mode that is bad.
type BadFileModeError FileMode

func (e BadFileModeError) Error() string {
	return fmt.Sprintf("DFS: Cannot perform this operation in current file mode [%s]", string(e))
}

// Contains filename.
type WriteModeTimeoutError string

func (e WriteModeTimeoutError) Error() string {
	return fmt.Sprintf("DFS: Write access to filename [%s] has timed out; reopen the file", string(e))
}

// Contains filename
type BadFilenameError string

func (e BadFilenameError) Error() string {
	return fmt.Sprintf("DFS: Filename [%s] includes illegal characters or has the wrong length", string(e))
}

// Contains filename
type FileUnavailableError string

func (e FileUnavailableError) Error() string {
	return fmt.Sprintf("DFS: Filename [%s] is unavailable", string(e))
}

// Contains local path
type LocalPathError string

func (e LocalPathError) Error() string {
	return fmt.Sprintf("DFS: Cannot access local path [%s]", string(e))
}

// Contains filename
type FileDoesNotExistError string

func (e FileDoesNotExistError) Error() string {
	return fmt.Sprintf("DFS: Cannot open file [%s] in D mode as it does not exist locally", string(e))
}

// </ERROR DEFINITIONS>
////////////////////////////////////////////////////////////////////////////////////////////

// Represents a file in the DFS system.
type DFSFile interface {
	// Reads chunk number chunkNum into storage pointed to by
	// chunk. Returns a non-nil error if the read was unsuccessful.
	//
	// Can return the following errors:
	// - DisconnectedError (in READ,WRITE modes)
	// - ChunkUnavailableError (in READ,WRITE modes)
	Read(chunkNum uint8, chunk *Chunk) (err error)

	// Writes chunk number chunkNum from storage pointed to by
	// chunk. Returns a non-nil error if the write was unsuccessful.
	//
	// Can return the following errors:
	// - BadFileModeError (in READ,DREAD modes)
	// - DisconnectedError (in WRITE mode)
	// - WriteModeTimeoutError (in WRITE mode)
	Write(chunkNum uint8, chunk *Chunk) (err error)

	// Closes the file/cleans up. Can return the following errors:
	// - DisconnectedError
	Close() (err error)
}

// Represents a connection to the DFS system.
type DFS interface {
	// Check if a file with filename fname exists locally (i.e.,
	// available for DREAD reads).
	//
	// Can return the following errors:
	// - BadFilenameError (if filename contains non alpha-numeric chars or is not 1-16 chars long)
	LocalFileExists(fname string) (exists bool, err error)

	// Check if a file with filename fname exists globally.
	//
	// Can return the following errors:
	// - BadFilenameError (if filename contains non alpha-numeric chars or is not 1-16 chars long)
	// - DisconnectedError
	GlobalFileExists(fname string) (exists bool, err error)

	// Opens a filename with name fname using mode. Creates the file
	// in READ/WRITE modes if it does not exist. Returns a handle to
	// the file through which other operations on this file can be
	// made.
	//
	// Can return the following errors:
	// - OpenWriteConflictError (in WRITE mode)
	// - DisconnectedError (in READ,WRITE modes)
	// - FileUnavailableError (in READ,WRITE modes)
	// - FileDoesNotExistError (in DREAD mode)
	// - BadFilenameError (if filename contains non alpha-numeric chars or is not 1-16 chars long)
	Open(fname string, mode FileMode) (f DFSFile, err error)

	// Disconnects from the server. Can return the following errors:
	// - DisconnectedError
	UMountDFS() (err error)
}

// The constructor for a new DFS object instance. Takes the server's
// IP:port address string as parameter, the localIP to use to
// establish the connection to the server, and a localPath path on the
// local filesystem where the client has allocated storage (and
// possibly existing state) for this DFS.
//
// The returned dfs instance is singleton: an application is expected
// to interact with just one dfs at a time.
//
// This call should succeed regardless of whether the server is
// reachable. Otherwise, applications cannot access (local) files
// while disconnected.
//
// Can return the following errors:
// - LocalPathError
// - Networking errors related to localIP or serverAddr
func MountDFS(serverAddr string, localIP string, localPath string) (dfs DFS, err error) {
	myclient = Clients{0, localIP, localPath, false}
	openedCounter = 0 // count opened files
	pathExists, _ := FileOrPathExists(localPath)
	if !pathExists {
		return nil, LocalPathError(localPath)
	}
	file := localPath + "/cid" // store client ID on disk
	clientIDExists, _ := FileOrPathExists(file)
	if clientIDExists {
		fl, err := os.Open(file)
		if err != nil {
			return nil, FileUnavailableError(file)
		}
		defer fl.Close()
		if err == nil {
			buf := make([]byte, 1)
			for {
				n, _ := fl.Read(buf)
				if 0 == n {
					break
				}
			}
			myclient.ID = int(buf[0])
			// fmt.Println("find cid", myclient.ID)
		}
	}
	DFSFileTables = make(map[string]MyDFSfile)
	openedFiles = make(map[int]openedFl)
	timeoutWrtFiles = make(map[string]bool)
	clRpc := new(ClientFunc)
	server := rpc.NewServer()
	registerServer(server, clRpc)

	l, e := net.Listen("tcp", localIP+":0") // listen server's rpc call
	if e != nil {
		log.Fatal("client listening error:", e)
		return nil, DisconnectedError("client listening fails")
	}

	port := l.Addr().(*net.TCPAddr).Port
	sp := strconv.Itoa(port)
	myclient.LocalIP = myclient.LocalIP + ":" + sp
	go server.Accept(l) //###
	runtime.Gosched()

	conn, err := rpc.Dial("tcp", serverAddr) // connect to server
	if err != nil {
		log.Println("fail to connect server:", err)
		// return nil, DisconnectedError("disconnect")
	} else {
		myConn = conn
		var reply int
		err1 := conn.Call("MyServer.Connect", myclient, &reply)
		if err1 != nil {
			return nil, DisconnectedError("disconnect")
		}
		myclient.Online = true
		if reply != 0 {
			// Store client ID in local disk
			myclient.ID = reply
			fout, err2 := os.Create(localPath + "/cid")
			if err2 != nil {
				// fmt.Println(localPath+"/cid", err2)
				return nil, FileUnavailableError("create client id fails")
			}
			defer fout.Close()
			buf := make([]byte, 1)
			buf[0] = byte(reply)
			fout.Write(buf)
		}
		go heartbeat()
	}
	var mydfs DFS = &myclient
	return mydfs, nil
}

func addtimeoutFl() {
	for n := range openedFiles {
		fl := openedFiles[n]
		if fl.Mode == WRITE {
			timeoutWrtFiles[fl.FileName] = true
		}
	}
}

func heartbeat() {
	for {
		reply := false
		_ = myConn.Call("MyServer.HeartbeatS", myclient.ID, &reply)
		time.Sleep(1600 * time.Millisecond)

		if reply {
			myclient.Online = true
		} else {
			addtimeoutFl()
			myclient.Online = false
		}
	}
}

var myclient Clients

type ClientFunc int
type ClientRpc interface {
	GrabChunk(args *TransChunk, reply *Chunk) error //[32]byte
	SetChunk(args *TransChunk, reply *bool) error
}

func registerServer(server *rpc.Server, s ClientRpc) {
	server.RegisterName("ClientRpc", s)
}

type TransChunk struct {
	Fname   string
	ChunkNo int
	Ver     int
	Buf     [32]byte
}

const ONECHUNK int = 32

func (c *ClientFunc) SetChunk(args *TransChunk, reply *bool) error {

	if DFSFileTables[args.Fname].FileName == "" {
		DFSFileTables[args.Fname] = MyDFSfile{myclient.ID, args.Fname, make(map[int]int), 0}
	}
	file := myclient.LocalPath + "/" + args.Fname + ".dfs"
	hasfile, _ := FileOrPathExists(file)
	// fmt.Println("r@53333=:", args.Fname, hasfile)
	if !hasfile {
		saveOnDisk(args.Fname, make([]byte, 8192))
	}
	fl, err := os.OpenFile(file, os.O_WRONLY, 0777)
	if err != nil {
		return FileUnavailableError(file)
	}
	defer fl.Close()
	if err == nil {
		var start = int64(args.ChunkNo * ONECHUNK)
		buf := args.Buf[:]
		n, err2 := fl.WriteAt(buf, start) // @err
		if err2 != nil {
			fmt.Println(n, err2)
			return FileUnavailableError(file)
		}
		DFSFileTables[args.Fname].Chunks[args.ChunkNo] = args.Ver
		return nil
	}
	return err
}

func (c *ClientFunc) GrabChunk(args *TransChunk, reply *Chunk) error {
	// fmt.Println("r@53322222222233=:")
	file := myclient.LocalPath + "/" + args.Fname + ".dfs"
	flExists, _ := FileOrPathExists(file)
	if flExists {
		fl, err := os.Open(file)
		if err != nil {
			// fmt.Println(file, err)
			return FileUnavailableError(file)
		}
		defer fl.Close()
		if err == nil {
			var start = int64(args.ChunkNo * ONECHUNK)
			buf := make([]byte, ONECHUNK)
			n, err2 := fl.ReadAt(buf, start)
			// fmt.Println("ReadChunk clt3", buf)
			copyData(reply, buf, n)
			if err2 != nil {
				// fmt.Println(file, err2)
				return ChunkUnavailableError(args.ChunkNo)
			}
			return nil

		}
	} else {
		return FileDoesNotExistError(file)
	}
	return nil
}

func copyData(to *Chunk, from []byte, total int) {
	for i := 0; i < total; i++ {
		to[i] = from[i]
	}
}

var myConn *rpc.Client

type Clients struct {
	ID        int
	LocalIP   string
	LocalPath string
	Online    bool
}

func (c *Clients) LocalFileExists(fname string) (exists bool, err error) {
	path := c.LocalPath + "/" + fname + ".dfs"

	return FileOrPathExists(path)
}

func (c *Clients) GlobalFileExists(fname string) (exists bool, err error) {
	var reply bool
	if !myclient.Online {
		return false, DisconnectedError("disconnect")
	}
	err1 := myConn.Call("MyServer.GlobalFileExists", fname, &reply)
	if err1 != nil {
		// fmt.Println(" error:", err1)
		return false, DisconnectedError("disconnect")
	}
	return reply, nil
}

// MyDFSfile use for RPC call to pass file's information
type MyDFSfile struct {
	ClientID int
	FileName string
	Chunks   map[int]int // record chunk version of each chunk
	ChunkNum int         // temporary argument for update chunk version
}

type openedFl struct {
	OpenID   int
	FileName string
	Mode     FileMode
}

var openedCounter int
var DFSFileTables map[string]MyDFSfile
var openedFiles map[int]openedFl

// Open to grab some available chunk version, get version information of itself from server
func (c *Clients) Open(fname string, mode FileMode) (f DFSFile, err error) {
	valid, err0 := validFileName(fname)
	if !valid {
		return nil, err0
	}
	if mode != DREAD && !c.Online {
		return nil, DisconnectedError(fname)
	}
	local, _ := c.LocalFileExists(fname)

	if mode == DREAD {
		if !local {
			return nil, FileDoesNotExistError(fname)
		}
		tmp := openedFl{openedCounter, fname, mode}
		openedFiles[openedCounter] = tmp
		openedCounter++
		var dfsfl DFSFile = &tmp
		return dfsfl, nil
	}

	// following for read/write mode
	globalExist, _ := c.GlobalFileExists(fname)

	if !globalExist {
		var dfsFl = MyDFSfile{c.ID, fname, make(map[int]int), 0}
		var reply bool
		if mode == WRITE {
			err := myConn.Call("MyServer.OpenNewFileWrtMode", dfsFl, &reply)
			if err != nil {
				// fmt.Println("error:", err)
				return nil, DisconnectedError(fname)
			}
			if !reply {
				return nil, OpenWriteConflictError(fname) // @other error
			}
			delete(timeoutWrtFiles, fname)
		} else {
			err := myConn.Call("MyServer.AddToGlobalExists", fname, &reply)
			if err != nil {
				return nil, DisconnectedError(mode)
			}
		}
		// fmt.Println("_+++llll++++file:", fname)
		DFSFileTables[fname] = dfsFl
		err3 := saveOnDisk(fname, make([]byte, 8192))
		if err3 != nil {
			return nil, err3
		}
	} else {
		var dfsFl = MyDFSfile{c.ID, fname, make(map[int]int), 0}
		if mode == WRITE {
			var reply bool
			err := myConn.Call("MyServer.OpenNewFileWrtMode", dfsFl, &reply) //###
			if err != nil {
				return nil, DisconnectedError(mode)
			}
			if !reply {
				return nil, OpenWriteConflictError(fname)
			}
			delete(timeoutWrtFiles, fname)
		}

		var updateMap MyDFSfile
		err := myConn.Call("MyServer.GrabFile", dfsFl, &updateMap)
		dfsFl.Chunks = updateMap.Chunks
		if updateMap.ChunkNum == 0 { // all other clients which have some chunk versions are offline
			return nil, FileUnavailableError(fname)
		}
		if err != nil {
			return nil, DisconnectedError(fname)
		}
		DFSFileTables[fname] = dfsFl
		// fmt.Println("recover version table", dfsFl)
	}

	tmp := openedFl{openedCounter, fname, mode}
	openedFiles[openedCounter] = tmp
	openedCounter++
	var dfstmp DFSFile = &tmp
	return dfstmp, nil
}

func (c *Clients) UMountDFS() (err error) {
	// release wrt lock, conn
	var reply bool
	if !myclient.Online {
		return DisconnectedError("UMountDFS client offline")
	}
	err1 := myConn.Call("MyServer.ReleaseAll", myclient.ID, &reply)
	if err1 != nil {
		// fmt.Println(err1, myclient.Online)
		return DisconnectedError("UMountDFS")
	}
	return nil
}

func readChunkFromDisk(file string, chunkNum int64) ([]byte, error) {
	fl, err := os.Open(file)
	if err != nil {
		// fmt.Println(file, err)
		return nil, FileUnavailableError(file)
	}
	defer fl.Close()
	var buf []byte

	var start = int64(chunkNum) * int64(ONECHUNK)
	buf = make([]byte, ONECHUNK)
	n, err2 := fl.ReadAt(buf, start)
	if err2 != nil {
		fmt.Println(n, err2)
		return nil, FileUnavailableError(file)
	}
	// fmt.Println(n, "readd", file, buf)
	return buf, nil
}

func writeChunkToDisk(fname string, buf *Chunk, chunkNum int64) error {
	file := myclient.LocalPath + "/" + fname + ".dfs"

	fl, err := os.OpenFile(file, os.O_WRONLY, 0777)
	if err != nil {
		fmt.Println(file, err)
		return FileUnavailableError(file)
	}

	defer fl.Close()
	var start = chunkNum * int64(ONECHUNK)
	buf1 := buf[:]
	n, err2 := fl.WriteAt(buf1, start)
	if err2 != nil {
		fmt.Println(n, err2)
		return FileUnavailableError(file)
	}
	return nil
}

type FileAlreadyClosed string

func (e FileAlreadyClosed) Error() string {
	return fmt.Sprintf("DFS: File [%s] is closed", string(e))
}

func (m *openedFl) Read(chunkNum uint8, chunk *Chunk) (err error) {
	file := myclient.LocalPath + "/" + m.FileName + ".dfs"
	if openedFiles[m.OpenID].FileName == "" {
		return FileAlreadyClosed(m.OpenID)
	}
	if m.Mode == DREAD {
		// local exist
		buf, err := readChunkFromDisk(file, int64(chunkNum))
		if err != nil {
			return err
		}
		copyData(chunk, buf, ONECHUNK)
		// fmt.Println(chunk)
		return nil
	}
	if !myclient.Online {
		return DisconnectedError("Read disconnect")
	}
	chkVer := DFSFileTables[m.FileName].Chunks[int(chunkNum)]
	args := MyDFSfile{myclient.ID, m.FileName, make(map[int]int), int(chunkNum)}
	args.Chunks[int(chunkNum)] = chkVer
	needUpdate := false
	err1 := myConn.Call("MyServer.ReadChunk", args, &needUpdate)
	if needUpdate {
		// fmt.Println("er3!!!!!!!!:", needUpdate)
		return ChunkUnavailableError(chunkNum)
	} else if err1 != nil {
		return DisconnectedError("Read chunk fail")
	}

	buf, err := readChunkFromDisk(file, int64(chunkNum))
	if err != nil {
		return err
	}
	copyData(chunk, buf, ONECHUNK)
	return nil
}

var timeoutWrtFiles map[string]bool

func (m *openedFl) Write(chunkNum uint8, chunk *Chunk) (err error) {

	if openedFiles[m.OpenID].FileName == "" {
		return FileAlreadyClosed(m.OpenID)
	}
	if m.Mode != WRITE {
		return BadFileModeError(m.Mode)
	}
	if timeoutWrtFiles[m.FileName] {
		return WriteModeTimeoutError(m.FileName)
	}
	//@CheckWrtTimeout(cid, bool)-> timeoutClients[int]true | openWrt->check timeoutClient and releaselock
	err2 := writeChunkToDisk(m.FileName, chunk, int64(chunkNum))
	if err2 != nil {
		return err2
	}
	var newVer int
	args := MyDFSfile{myclient.ID, m.FileName, nil, int(chunkNum)}
	err1 := myConn.Call("MyServer.WriteChunck", args, &newVer)
	if err1 != nil {
		return DisconnectedError("write")
	}
	if nil == DFSFileTables[m.FileName].Chunks {
		strct := DFSFileTables[m.FileName]
		strct.Chunks = make(map[int]int)
		DFSFileTables[m.FileName] = strct
	}
	DFSFileTables[m.FileName].Chunks[int(chunkNum)] = newVer
	// fmt.Println(newVer)
	return nil
}

func (m *openedFl) Close() (err error) {
	// fmt.Println(DFSFileTables)
	if m.Mode == DREAD {
		delete(openedFiles, m.OpenID)
		return nil
	}
	var err1 error = nil
	if !myclient.Online {
		return DisconnectedError("close()")
	}
	if m.Mode == WRITE {
		var reply bool
		args := MyDFSfile{myclient.ID, m.FileName, nil, 0}
		err1 = myConn.Call("MyServer.ReleaseWrt", args, &reply)
	}
	delete(openedFiles, m.OpenID)
	return err1
}

func saveOnDisk(fname string, data []byte) error {
	// fmt.Println("save on disk_+++++++")
	localPath := myclient.LocalPath
	file := localPath + "/" + fname + ".dfs"
	fout, err2 := os.Create(file)
	if err2 != nil {
		fmt.Println(file, err2)
		return FileUnavailableError(fname)
	}
	defer fout.Close()
	if len(data) != 0 {
		fout.Write(data)
	}
	return nil
}

func validFileName(fname string) (valid bool, err error) {
	strlen := len(fname)
	if strlen > 0 && strlen <= 16 {
		for i := 0; i < strlen; i++ {
			var s = string(fname[i : i+1])
			matched, _ := regexp.MatchString("[a-z]+|[0-9]+", s)
			if !matched {
				return false, BadFilenameError(fname)
			}
			// fmt.Println(matched, s)
		}
		return true, nil
	}
	return false, BadFilenameError(fname)
}

func FileOrPathExists(path string) (exists bool, err error) {
	fi, err := os.Stat(path)

	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	if fi == nil {
		return false, nil
	}
	return false, err
}

func CreateLocalPath(localPatch string) {
	fm := os.FileMode(0777)
	var path string
	if localPatch[len(localPatch)-1] == '/' {
		path = localPatch[:len(localPatch)-1]
		// fmt.Println("??33 " + path)
	} else {
		path = localPatch
	}
	err1 := os.MkdirAll(path, fm)
	if err1 != nil {
		panic(err1)
	}
}
