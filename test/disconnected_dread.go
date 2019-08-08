package main

// Expects dfslib.go to be in the ./dfslib/ dir, relative to
// this app.go file
import (
	"fmt"
	"os"
	"time"

	"../dfslib"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Println("Need Server address [ip:port]")
		return
	}

	serverAddr := os.Args[1]
	localIP := "127.0.0.1"
	localPath := "../tmp_disconnected_dread/"
	dfslib.CreateLocalPath(localPath)
	// Connect to DFS.
	dfs, err := dfslib.MountDFS(serverAddr, localIP, localPath)

	if checkError(err) != nil {
		return
	}
	printTime()
	fmt.Println("A's DFS mounted")

	printTime()
	fmt.Println("A opening file 'thefile' for WRITE")
	f, err := dfs.Open("thefile", dfslib.WRITE)
	if checkError(err) != nil {
		fmt.Println("File err", err)
		return
	}

	var chunk dfslib.Chunk
	str := "Hello from CPSC 416!"
	chunk = stringToChunk(str)

	// Write the 3rd chunk of the file.
	err = f.Write(3, &chunk)
	if checkError(err) != nil {
		return
	}
	printTime()
	fmt.Println("A Writing chunk 3: ", str)

	printTime()
	fmt.Println("A CLOSE file 'thefile'")
	f.Close()

	fmt.Println("please disconnect")
	time.Sleep(8 * time.Second)

	printTime()
	fmt.Println("A opening file 'thefile' for DREAD")
	f, err = dfs.Open("thefile", dfslib.DREAD)
	if checkError(err) != nil {
		return
	}

	var chunk2 dfslib.Chunk
	err = f.Read(3, &chunk2)
	if checkError(err) != nil {
		return
	}
	s := string(chunk2[:])
	printTime()
	fmt.Println("A Read from chunk 3:", s)

	printTime()
	fmt.Println("A CLOSE file 'thefile'")
	f.Close()

	printTime()
	fmt.Println("A UMountDFS")
	err = dfs.UMountDFS()
	if checkError(err) != nil {
		return
	}
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
