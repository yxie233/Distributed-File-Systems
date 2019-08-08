package main

// Expects dfslib.go to be in the ./dfslib/ dir, relative to
// this app.go file
import (
	"fmt"
	"os"
	"time"

	"../../dfslib"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Println("Need Server address [ip:port]")
		return
	}

	serverAddr := os.Args[1]
	localIP := "127.0.0.1"
	localPath := "../../tmp_open_same_file_for_writing/clientB"

	dfslib.CreateLocalPath(localPath)

	// Connect to DFS.
	dfs, err := dfslib.MountDFS(serverAddr, localIP, localPath)

	if checkError(err) != nil {
		return
	}
	printTime()
	fmt.Println("B's DFS mounted")

	time.Sleep(1500 * time.Millisecond)
	printTime()
	fmt.Println("B opening file 'thefile' for WRITE")
	f, err := dfs.Open("thefile", dfslib.WRITE)
	if checkError(err) == nil {
		f.Close()
	}

	defer dfs.UMountDFS()
	printTime()
	fmt.Println("B UMountDFS")
}

// If error is non-nil, print it out and return it.
func checkError(err error) error {
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error ", err.Error())
		return err
	}
	return nil
}

func stringToChunk(str string) dfslib.Chunk {
	var chunk dfslib.Chunk
	// const str3 = str
	copy(chunk[:], str)
	len1 := len(str)
	for i := len1; i < 32; i++ {
		chunk[i] = 0
	}
	return chunk
}

func printTime() {
	t := time.Now()
	fmt.Print(t.Format("[2006-01-02/15:04:05] "))
}
