package main

import (
	"fmt"
	"log"
	"net"
	"net/rpc"
	"os"
	"sync"
	"time"
)

type Chunk [32]byte
type Clients struct {
	ID        int
	LocalIP   string
	LocalPath string
	Online    bool
}

type MyDFSfile struct {
	ClientID int
	FileName string
	Chunks   map[int]int
	ChunkNum int
}

// RPC functions for clients
type ServerFunc interface {
	Connect(args *Clients, reply *int) error
	GlobalFileExists(fname *string, reply *bool) error
	OpenNewFileWrtMode(args *MyDFSfile, reply *bool) error
	GrabFile(args *MyDFSfile, reply *MyDFSfile) error
	AddToGlobalExists(fname *string, reply *bool) error
	ReadChunk(args *MyDFSfile, reply *bool) error
	WriteChunck(args *MyDFSfile, ver *int) error
	ReleaseWrt(args *MyDFSfile, reply *bool) error
	HeartbeatS(cid *int, reply *bool) error
	ReleaseAll(cid *int, reply *bool) error
}

type MyServer int

type MetaData struct {
	version    int                  // latest chunk version
	chkVerData map[int]map[int]bool // {v1 :{1:true, 2:true, 5:true}, v2:{3:true} }
}

type TransChunk struct {
	Fname   string
	ChunkNo int
	Ver     int
	Buf     [32]byte
}

type ClientRcd struct {
	ClientID        int
	LocalIP         string
	Conn            *rpc.Client
	Online          bool
	RecentHeartbeat int64
}

var records clientsAndFiles = clientsAndFiles{
	globalFileExistTable: make(map[string]bool),
	fileInWtrTable:       make(map[string]int),
	knownClients:         make(map[int]ClientRcd),
	latestVersion:        make(map[string]map[int]MetaData)}

type clientsAndFiles struct {
	sync.RWMutex
	clientIDRecord       int // record next unique ID in order to assign new client
	globalFileExistTable map[string]bool
	fileInWtrTable       map[string]int              // which file is writing by which client
	latestVersion        map[string]map[int]MetaData // "fl": {chk0 : {ver, rcds}, ..}
	knownClients         map[int]ClientRcd
}

type ChunkUnavailableError uint8

func (e ChunkUnavailableError) Error() string {
	return fmt.Sprintf("DFS: Latest verson of chunk [%s] unavailable", string(e))
}

type InvalidRequestError string

func (e InvalidRequestError) Error() string {
	return fmt.Sprintf("DFS: The request is invalid [%s]", string(e))
}

func (t *MyServer) Connect(args *Clients, reply *int) error {
	records.Lock()
	defer records.Unlock()
	var cid = args.ID
	if args.ID == 0 {
		records.clientIDRecord++
		fmt.Println("assign id to new client: ", records.clientIDRecord)
		*reply = records.clientIDRecord
		cid = records.clientIDRecord
	} else {
		*reply = 0
	}

	x, err0 := rpc.Dial("tcp", args.LocalIP)
	if err0 != nil {
		return err0
	}

	records.knownClients[cid] = ClientRcd{cid, args.LocalIP, x, true, time.Now().UnixNano()}

	go monitor(cid, time.Duration(2000)*time.Millisecond) // @set check time

	fmt.Println("Got connect from client", cid, args.LocalIP)
	return nil
}

// Function to delete dead client (no recent heartbeat)
func monitor(cid int, heartBeatInterval time.Duration) {
	for {
		records.Lock()

		if time.Now().UnixNano()-records.knownClients[cid].RecentHeartbeat > int64(heartBeatInterval) {
			fmt.Println("client timed out: ", records.knownClients[cid].LocalIP)
			clt := records.knownClients[cid]
			clt.Online = false
			records.knownClients[cid] = clt
			// fmt.Println("after:", cid, records.knownClients[cid].Online)
			records.Unlock()
			return
		}
		fmt.Println("client", cid, "is alive", records.knownClients[cid].LocalIP)
		records.Unlock()
		time.Sleep(heartBeatInterval)
	}
}

// release client's all files in write table
func releaseWrts(cid int) {
	records.Lock()
	defer records.Unlock()
	for fn, wid := range records.fileInWtrTable {
		// fmt.Println("rls all file ", fn, wid)
		if wid == cid {
			delete(records.fileInWtrTable, fn)
		}
	}
}

func (t *MyServer) ReleaseAll(cid *int, reply *bool) error {
	id := *cid
	tmp := records.knownClients[id]
	tmp.Conn = nil
	tmp.Online = false
	records.knownClients[id] = tmp
	releaseWrts(id)
	return nil
}

func (t *MyServer) GlobalFileExists(fname *string, reply *bool) error {
	*reply = records.globalFileExistTable[*fname]
	return nil
}

func (t *MyServer) AddToGlobalExists(fname *string, reply *bool) error {
	records.Lock()
	defer records.Unlock()
	records.globalFileExistTable[*fname] = true
	*reply = true
	return nil
}

func (t *MyServer) OpenNewFileWrtMode(args *MyDFSfile, reply *bool) error {
	records.Lock()
	defer records.Unlock()
	wrtID := records.fileInWtrTable[args.FileName]
	// fmt.Println("open wrt file id", wrtID, records.knownClients[wrtID].Online)
	if 0 != wrtID && wrtID != args.ClientID && records.knownClients[wrtID].Online {
		*reply = false
		return nil
	}
	records.fileInWtrTable[args.FileName] = args.ClientID
	records.globalFileExistTable[args.FileName] = true
	*reply = true
	return nil
}

func (t *MyServer) ReleaseWrt(args *MyDFSfile, reply *bool) error {
	records.Lock()
	defer records.Unlock()
	if records.fileInWtrTable[args.FileName] == args.ClientID {
		records.fileInWtrTable[args.FileName] = 0
		return nil
	}
	return InvalidRequestError("No permission to relase write lock of" + args.FileName)
}

func (t *MyServer) HeartbeatS(cid *int, reply *bool) error {
	records.Lock()
	defer records.Unlock()

	*reply = true //###
	clt := records.knownClients[*cid]
	clt.RecentHeartbeat = time.Now().UnixNano()
	records.knownClients[*cid] = clt
	return nil
}

func (t *MyServer) WriteChunck(args *MyDFSfile, ver *int) error {
	newVer := records.latestVersion[args.FileName][args.ChunkNum].version
	newVer++
	if nil == records.latestVersion[args.FileName] {
		records.latestVersion[args.FileName] = make(map[int]MetaData)
	}
	strct := records.latestVersion[args.FileName][args.ChunkNum]
	strct.version = newVer
	records.latestVersion[args.FileName][args.ChunkNum] = strct
	if nil == strct.chkVerData {
		strct.chkVerData = make(map[int]map[int]bool) // {ch0: {v1:{cl1:true, cl5:true}}, ch2..}
	}
	if nil == strct.chkVerData[newVer] {
		strct.chkVerData[newVer] = make(map[int]bool)
	}
	strct.chkVerData[newVer][args.ClientID] = true
	records.latestVersion[args.FileName][args.ChunkNum] = strct
	*ver = newVer
	return nil
}

// only read the most up-to-date version, if not available then return error
func (t *MyServer) ReadChunk(args *MyDFSfile, needUpdate *bool) error {
	latestVer := records.latestVersion[args.FileName][args.ChunkNum].version

	if args.Chunks[args.ChunkNum] < latestVer {
		data := records.latestVersion[args.FileName][args.ChunkNum].chkVerData
		clients := data[latestVer]
		gotChunk := false
		for j := range clients {

			if records.knownClients[j].Online {
				var buf [32]byte
				f := TransChunk{args.FileName, args.ChunkNum, latestVer, buf}
				err1 := records.knownClients[j].Conn.Call("ClientRpc.GrabChunk", f, &buf)
				if err1 != nil {
					// fmt.Println("error@##:", err1)
					continue
				}
				fmt.Println("ReadChunk clt", args.ClientID, buf)
				toSet := TransChunk{args.FileName, args.ChunkNum, latestVer, buf}
				var rst bool
				err2 := records.knownClients[args.ClientID].Conn.Call("ClientRpc.SetChunk", toSet, &rst)
				if err2 != nil {
					// fmt.Println("error#$:", err2)
					break
				}
				// fmt.Println("gotChunk clt", j)
				tmp := records.latestVersion[args.FileName][args.ChunkNum]
				data1 := records.latestVersion[args.FileName][args.ChunkNum].chkVerData
				clients1 := data1[latestVer]
				clients1[args.ClientID] = true
				data1[latestVer] = clients1
				tmp.chkVerData = data1
				records.latestVersion[args.FileName][args.ChunkNum] = tmp
				gotChunk = true
				break
			}
		}
		if !gotChunk {
			*needUpdate = true
			return ChunkUnavailableError(int8(args.ChunkNum))
		}
	}
	return nil
}

// grab some avaliable chunks of given file
func (t *MyServer) GrabFile(args *MyDFSfile, reply *MyDFSfile) error {
	fl := records.latestVersion[args.FileName] // {chk0: metadata, ..}
	reply.Chunks = args.Chunks
	gotSomeVersion := false
	if nil == fl {
		fl = make(map[int]MetaData)
	}
	if len(fl) == 0 {
		// fmt.Println("zero!", args, fl)
		gotSomeVersion = true
	}
	for chk, value := range fl { // each non-trivial chunk
		// fmt.Println("recover 11", chk)
		for k := value.version; k > 0; k-- { // eg. k=v3, if v3 not online then try v2
			clients := value.chkVerData[k] // version k: 1.2.15.3.
			if clients[args.ClientID] {
				gotSomeVersion = true
				reply.Chunks[chk] = k
				break
			}
			if args.Chunks[chk] >= k { // if current version is latest compared to other online clients, then just check next chunk
				gotSomeVersion = true
				break
			}

			for j := range clients { // grab chunk from online client
				// fmt.Println("online cid", j, records.knownClients[j].Online, args.ClientID)
				if records.knownClients[j].Online && records.knownClients[j].Conn != nil {
					var buf [32]byte
					f := TransChunk{args.FileName, chk, k, buf} //@ chk Filename
					err1 := records.knownClients[j].Conn.Call("ClientRpc.GrabChunk", f, &buf)
					if err1 != nil {
						fmt.Println(" error@1:", err1)
						return err1
					}
					toSet := TransChunk{args.FileName, chk, k, buf}
					var rst bool
					err2 := records.knownClients[args.ClientID].Conn.Call("ClientRpc.SetChunk", toSet, &rst)
					// fmt.Println("r@565=:", buf)
					if err2 != nil {
						fmt.Println(" error!@:", err2)
						return err2
					}

					tmp := records.latestVersion[args.FileName][chk]
					data1 := records.latestVersion[args.FileName][chk].chkVerData
					clients1 := data1[k]
					clients1[args.ClientID] = true
					data1[k] = clients1
					tmp.chkVerData = data1
					records.latestVersion[args.FileName][chk] = tmp
					gotSomeVersion = true
					break
				}
			}
		}
	}
	if gotSomeVersion {
		reply.ChunkNum = 1 // use 1 to tell client already get some version
	}
	return nil
}

func registerServer(server *rpc.Server, s ServerFunc) {
	// registers interface by name of `MyServer`.
	server.RegisterName("MyServer", s)
}

func main() {
	if len(os.Args) != 2 {
		fmt.Println("Need Server address [ip:port]")
		return
	}

	serverAddr := os.Args[1]
	serverFunc := new(MyServer)
	server := rpc.NewServer()
	registerServer(server, serverFunc)

	// Listen for incoming tcp packets on specified port.
	l, e := net.Listen("tcp", serverAddr)
	if e != nil {
		log.Fatal("listen error:", e)
	}
	// port := l.Addr().(*net.TCPAddr).IP
	// fmt.Println(port)
	server.Accept(l)
}
