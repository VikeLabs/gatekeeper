psql:
	docker exec -it gatekeeper_postgres_1 psql -U postgres -w postgres  
table:
	cat db.sql | docker exec -i gatekeeper_postgres_1 psql -U postgres -w postgres  