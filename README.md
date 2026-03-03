# codeqlAI

## codeql-cli安装
```bash
# 下载codeql压缩包
wget https://github.com/github/codeql-cli-binaries/releases/download/v2.24.2/codeql-linux64.zip -O /tmp/codeql-linux64.zip

unzip /tmp/codeql-linux64.zip -d /opt/codeql
# 修正目录权限（避免后续执行codeql命令无权限）
chown -R $USER:$USER /opt/codeql
```
```bash
# 打开配置文件
vi ~/.bashrc
# 添加以下内容（替换为你的解压路径）
export PATH="$PATH:/opt/codeql/codeql"
# 生效配置
source ~/.bashrc

# 或通过软链映射
ln -s /opt/codeql/codeql /bin/codeql
```


## 启动数据库容器
```bash
podman-compose up -d
```

## 后端服务启动
```bash
go run ./cmd/server
```

## 前端服务启动
```bash
cd web && npm install && npm run dev
```