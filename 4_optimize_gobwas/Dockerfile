FROM golang:1.21-alpine

WORKDIR /app

# 필요한 시스템 패키지 설치
RUN apk add --no-cache gcc musl-dev

# Go 모듈 파일 복사 및 의존성 다운로드
COPY go.mod go.sum ./
RUN go mod download

# 소스 코드 복사
COPY . .

# 애플리케이션 빌드
RUN go build -o websocket-server .

# 포트 노출
EXPOSE 8080 6060

# 실행 권한 설정
RUN chmod +x websocket-server

# root 사용자로 실행
USER root

# 애플리케이션 실행
CMD ["./websocket-server"] 