# slex

[![Build Status](https://travis-ci.org/hzxiao/slex.svg?branch=master)](https://travis-ci.org/hzxiao/slex)



## 安装

### 编译源代码

```shell
go get https://github.com/hzxiao/slex.git
go install $GOPATH/github.com/hzxiao/slex/cmd/slex
```

### 下载最新二进制文件

[https://github.com/hzxiao/slex/releases]: https://github.com/hzxiao/slex/releases



## 用法

将`slex`和`slex-srv.yaml`放到公司具有公网的IP的机器上。

将`slex`和`slec-cli.yaml`放在本机器上。

### 通过ssh访问公司内部机器

1. 修改`slex-srv.yaml`文件

   ```yaml
   name: Server
   listen: :11089
   access:
   -
     name: Client
     token: 123456
   ```

2. 运行放到公司具有公网的IP的机器上的`slex`

   ```shell
   slex -s -f slex-srv.yaml
   ```

3. 修改`slex-cli.yaml`文件，公司公网IP为x.x.x.x, 内外IP为10.0.0.x

   ```yaml
   name: Client
   channels:
     -
       name: Server
       enable: true
       token: 123456
       remote: x.x.x.x:11089
   forwards:
     -
       local: :8890
       route: Server->tcp://10.0.0.y:22
   ```

4. 运行本机的`slex`

   ```shell
   slex -c -f slex-cli.yaml
   ```

5. 使用ssh访问内部机器

   ```shell
   ssh -oPort=8890 root@localhost
   ```

   

