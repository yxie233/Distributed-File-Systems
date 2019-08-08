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
	localPath := "../tmp_disconnected_local_file_exists"
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
	const str = "Hello from CPSC 416!"
	copy(chunk[:], str)
	printTime()
	fmt.Println("A Writing chunk 3: ", str)

	err = f.Write(3, &chunk)
	if checkError(err) != nil {
		return
	}
	printTime()
	fmt.Println("A CLOSE file 'thefile'")
	f.Close()

	fmt.Println("Please disconnect")
	time.Sleep(8 * time.Second)

	exist, err := dfs.LocalFileExists("thefile")
	printTime()
	fmt.Println("local file 'thefile' exist: ", exist)

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

func printTime() {
	t := time.Now()
	fmt.Print(t.Format("[2006-01-02/15:04:05] "))
}
