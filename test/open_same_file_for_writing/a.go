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
	localPath := "../../tmp_open_same_file_for_writing/clientA"

	dfslib.CreateLocalPath(localPath)

	// Connect to DFS.
	dfs, err := dfslib.MountDFS(serverAddr, localIP, localPath)

	if checkError(err) != nil {
		return
	}
	printTime()
	fmt.Println("A's DFS mounted")
	time.Sleep(1000 * time.Millisecond)
	printTime()
	fmt.Println("A opening file 'thefile' for WRITE")
	f, err := dfs.Open("thefile", dfslib.WRITE)
	if checkError(err) != nil {
		fmt.Println("File err", err)
		return
	}

	str := "drww3data1"
	chunk := stringToChunk(str)

	printTime()
	fmt.Println("A Writing chunk 2: ", str)
	err = f.Write(2, &chunk)
	if checkError(err) != nil {
		return
	}

	time.Sleep(5000 * time.Millisecond)
	printTime()
	fmt.Println("A close file 'thefile'")
	f.Close()
	defer dfs.UMountDFS()
	printTime()
	fmt.Println("A UMountDFS")
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
