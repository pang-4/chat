### 문제 
PostgreSQL 접속을 시도하였을 때 아래 오류가 발생하였다. 
```
text FATAL: database "chat" does not exist 
``` 
Docker 컨테이너는 정상 실행 중이었지만 `chat` 데이터베이스를 찾지 못하였다. 

---
###  원인 분석 
Docker 로그에서는 최초 생성 시 
```
text CREATE DATABASE
``` 
가 출력되었지만, 현재 실행 중인 PostgreSQL에서는 `chat` 데이터베이스가 존재하지 않았다.

이는 기존 Volume에 저장된 데이터와 현재 compose.yml 설정이 일치하지 않아 초기 설정이 제대로 반영되지 않은 상태였다.

---
### 해결 과정 
Volume을 삭제한 뒤 PostgreSQL을 다시 생성하였다. 
``` bash 
docker compose down -v 
docker compose up -d 
``` 
이후 PostgreSQL 내부에 접속하여 데이터베이스를 확인하였다. 
```
bash docker exec -it chat-postgres psql -U postgres 
``` 

```
sql \l 
``` 
---
### 결과 
데이터베이스 목록에서 ```text chat postgres template0 template1 ``` 을 확인하였으며, Go 서버에서도 정상적으로 해당 데이터베이스에 연결할 수 있었다.

---
### 배운 점 
Docker Compose에서 설정한 ```yaml POSTGRES_DB ``` 는 컨테이너 최초 생성 시에만 적용된다. Volume이 남아 있는 상태에서는 compose.yml을 수정해도 데이터베이스가 자동으로 생성되지 않는다.
PostgreSQL 초기 설정을 변경해야 하는 경우에는 기존 Volume까지 함께 제거해야 한다.