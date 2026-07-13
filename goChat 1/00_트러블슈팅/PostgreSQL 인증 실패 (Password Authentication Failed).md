### 문제 Go 서버에서 PostgreSQL 연결을 시도했을 때 아래와 같은 오류가 발생하였다. 
```
text FATAL: password authentication failed for user "postgres" 
``` 
DB 서버는 실행 중이었지만 인증에 실패하여 애플리케이션이 PostgreSQL에 연결되지 않았다. 

---
###  원인 
분석 처음에는 `.env`의 비밀번호가 잘못되었거나 PostgreSQL 설정 문제라고 생각하였다. 
하지만 Docker 로그와 PostgreSQL 로그를 확인한 결과, 
```
text Connection matched file ... password authentication failed for user "postgres" 
``` 
오류가 반복적으로 발생하는 것을 확인하였다. 
추가로 PostgreSQL Docker 이미지는 - POSTGRES_USER - POSTGRES_PASSWORD - POSTGRES_DB 환경변수를 컨테이너 최초 생성 시에만 적용하며, 이미 생성된 Volume이 존재하면 이후 compose.yml을 수정해도 기존 정보가 유지된다는 점을 확인하였다. 

--- 
### 해결 과정 
기존 PostgreSQL Volume을 삭제하여 초기화를 진행하였다. 
```
bash docker compose down -v 
```
 이후 PostgreSQL을 다시 생성하였다. 
 ```
 bash docker compose up -d 
 ``` 
 Volume까지 삭제하면서 새로운 계정과 비밀번호가 정상적으로 생성되었다. 
 
  ---

### 결과
  Go에서 PostgreSQL 인증이 정상적으로 완료되었으며 더 이상 인증 오류가 발생하지 않았다. 
  
  ---  
### 배운 점 
Docker Volume은 데이터를 영구 저장하기 때문에 컨테이너를 삭제하더라도 초기 계정 정보는 유지된다. 
PostgreSQL의 - POSTGRES_USER - POSTGRES_PASSWORD - POSTGRES_DB 설정은 최초 생성 시에만 적용된다는 점을 이해하게 되었다. 
개발 초기에는 
```
docker compose down -v 
``` 
를 사용하여 Volume까지 삭제해야 초기 설정을 다시 적용할 수 있다.