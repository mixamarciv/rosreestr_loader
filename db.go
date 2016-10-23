package main

import (
	"database/sql"
	//"strconv"

	_ "github.com/nakagami/firebirdsql"

	s "strings"

	mf "github.com/mixamarciv/gofncstd3000"
)

var db *sql.DB

type NullString struct {
	sql.NullString
}

func (p *NullString) get(defaultval string) string {
	if p.Valid {
		return p.String
	}
	return defaultval
}

var db_codepage = "UTF-8"

func Initdb() {
	/*************
	path, _ := mf.AppPath()
	path = s.Replace(path, "\\", "/", -1) + "/db/DB1.FDB"
	//path = "192.168.1.10:3050/D:/_db_web/db002/0002.FDB"
	//dbopt := "sysdba:masterkey@127.0.0.1:3050/" + path
	dbopt := "sysdba:masterkey@127.0.0.1:3050/F:/program/programming_go/projects/uzkh_martini_app/db/DB1.FDB"
	var err error
	db, err = sql.Open("firebirdsql", dbopt)
	LogPrintErrAndExit("ошибка подключения к базе данных "+dbopt, err)
	LogPrint("установлено подключение к БД: " + dbopt)

	db.SetMaxOpenConns(200)
	db.SetMaxIdleConns(100)

	query := `SELECT CAST(COUNT(*) AS VARCHAR(100)) FROM itemtype `
	rows, err := db.Query(query)
	rows.Next()
	var cnt string
	err = rows.Scan(&cnt)
	LogPrintErrAndExit("rows.Scan error: \n"+query+"\n\n", err)
	LogPrint("всего элементов в БД: " + cnt)
	**************/
	db_pass := "masterkey"
	path, _ := mf.AppPath()
	path = s.Replace(path, "\\", "/", -1) + "/db/DB1.FDB"
	//path = "d/program/go/projects/test_martini_app/db/DB1.FDB"
	//dbopt := "sysdba:" + db_pass + "@127.0.0.1:3050/" + path
	dbopt := "sysdba:" + db_pass + "@192.168.1.10:3050/d:/_db_web/db002/0002.fdb"
	var err error
	db, err = sql.Open("firebirdsql", dbopt)
	LogPrintErrAndExit("ошибка подключения к базе данных "+dbopt, err)
	LogPrint("установлено подключение к БД: " + dbopt)

	db.SetMaxOpenConns(200)
	db.SetMaxIdleConns(100)

	query := `SELECT COUNT(*) FROM rdb$database`
	rows, err := db.Query(query)
	LogPrintErrAndExit("db.Query error: \n"+query+"\n\n", err)
	rows.Next()
	var cnt string
	err = rows.Scan(&cnt)
	LogPrintErrAndExit("rows.Scan error: \n"+query+"\n\n", err)
	LogPrint("всего записей в БД: " + cnt)
}
