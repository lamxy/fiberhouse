# build or run cmd command base on COMMAND directory
- ### Configure based on the COMMAND directory or absolute path, otherwise the operation will panic

### go run
```shell
cd command/   # command ROOT Directory
go run /path/to/main.go
```

### go build
```shell
cd command/  # command ROOT Directory
# windows环境构建产物保留.exe后缀，linux环境无需保留后缀
go build -o ./target/cmdstarter.exe ./main.go 
```

### exec
```shell
cd command/    ## work dir is ~/command/, configure path base on it
./target/cmdstarter.exe -h
# or Linux、MacOS 环境
./target/cmdstarter -h
```