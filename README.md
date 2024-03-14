To create the corresponding binaries

**arm64**<br>
env GOOS=linux GOARCH=arm64 go build -o sample_cli_arm64                          

**x86_64**<br>
env GOOS=windows GOARCH=amd64 go build -o sample_cli_amd64
