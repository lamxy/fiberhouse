# build or run main web base on Project ROOT Directory
- ### Taking Windows environment as an example, Linux is similar
- ### Configure based on the COMMAND directory or absolute path, otherwise the operation will panic

### go run
```shell
cd PRD/   # Project ROOT Directory
go run /path/to/main.go
```

### go build
```shell
cd PRD/   # Project ROOT Directory
# windows环境构建产物保留.exe后缀，linux环境无需保留后缀
go build "-ldflags=-X 'main.Version=v0.0.1'" -o ./example_main/target/examplewebserver.exe ./example_main/main.go
```

### exec
```shell
cd PRD/    ## work dir is ~/`Project ROOT directory`/, configure path base on it.
./example_main/target/examplewebserver.exe
# or Linux、MacOS 环境
./example_main/target/examplewebserver
```