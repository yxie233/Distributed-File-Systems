The tests are work according senarios in the test\senario fold. 

How to run:
1.first run server(e.g. go run .\server.go 127.0.0.1:9999).
2.then run test, cd into the test folder and enter [go run filename.go IP:port](e.g. go run .\single_client.go 127.0.0.1:9999).

You'll need shut down the server and restart it when you run a different test.

You may want to change [localIP := "127.0.0.1" // it's used for the client's IP] of each test file when you're testing.

If a test contains more than one client, then you may quickly run a.go, b.go, c.go in order(e.g. first run a.go, then run b.go as soon as possible).

If you see "please disconnect", then you may shut down the server to simulate client disconnect with server.

Test example:
![example](https://github.com/yxie233/graphStore/blob/master/example_for_test_DFS/multiple_reader_mult_wrt_test.JPG)
