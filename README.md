# TMDB 反向代理服务器

## 使用方法

### Docker 部署

```bash
docker run -p 8080:8080 taurusxin/tmdb-proxy-go
```

服务启动在 8080 端口

### 手动部署

编译应用程序

```bash
go build
```

启动服务（默认8080端口）

```bash
./tmdb-proxy
```

指定端口启动服务

```bash
./tmdb-proxy -port 9090
```

查看帮助信息

```bash
./tmdb-proxy -help
```