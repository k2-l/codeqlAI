# codeqlAI


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