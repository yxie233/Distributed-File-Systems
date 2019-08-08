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
	localPath := "../../tmp_multiple_reader_multiple_writer/clientC"
	dfslib.CreateLocalPath(localPath)

	// Connect to DFS.
	dfs, err := dfslib.MountDFS(serverAddr, localIP, localPath)

	if checkError(err) != nil {
		return
	}
	printTime()
	fmt.Println("C's DFS mounted")
	time.Sleep(1000 * time.Millisecond)
	printTime()
	fmt.Println("C opening file 'thefile' for READ")
	f, err := dfs.Open("thefile", dfslib.READ)
	if checkError(err) != nil {
		return
	}
	time.Sleep(2000 * time.Millisecond)
	var chunk dfslib.Chunk

	// READ the 3rd chunk of the file.
	err = f.Read(3, &chunk)
	if checkError(err) != nil {
		return
	}
	s := string(chunk[:])
	printTime()
	fmt.Println("C Read chunk 3 from file 'thefile': ", s)

	f.Close()
	time.Sleep(5000 * time.Millisecond)

	printTime()
	fmt.Println("C opening file 'thefile' for READ")
	f1, err := dfs.Open("thefile", dfslib.READ)
	if checkError(err) != nil {
		return
	}

	// READ the 3rd chunk of the file.
	err = f1.Read(3, &chunk)
	if checkError(err) != nil {
		return
	}
	s = string(chunk[:])
	printTime()
	fmt.Println("C Read chunk 3 from file 'thefile': ", s)
	time.Sleep(3000 * time.Millisecond)
	printTime()
	fmt.Println("C close file")
	f1.Close()

	printTime()
	fmt.Println("C UMountDFS")
	defer dfs.UMountDFS()
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
