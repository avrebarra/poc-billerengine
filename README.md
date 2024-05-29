# poc/billerengine

## Quickstart

```sh
# setup sqlite database
python .scripts/exec_sql.py --file=migration.sql --sqliteurl=db.sqlite

# start services
go run .
```

## Notes

There are implementation details that are intentionally left out for the sake of rapid development:

- Configs (PORT & DB) should be extracted to files or centralized config mgr instead of written as constants
- Service auth scheme should be implemented
- Storage layer should be implemented as separate layer to implement more capable DBMS (Postgre/MySQL)
- Should add more thorough unit tests for each engine functions
