// database.go
package main

import (
	"database/sql"
	"fmt"

	_ "github.com/go-sql-driver/mysql"
	//	"runtime"
	"strings"
	"time"
)

type DBHander struct {
	db         *sql.DB
	err        error
	jsonstring string
	cache      *CacheHander
}

func (this *DBHander) Init(db_ip string, cache_ports []string, node_ips []string) error {
	this.cache = &CacheHander{cache_ports: cache_ports, node_ips: node_ips}
	this.cache.Init()

	this.db, this.err = sql.Open("mysql", "xiaoxia:foxitqa123@tcp("+db_ip+":3306)/testfileserver?charset=utf8")
	if this.err != nil {
		fmt.Println(this.err.Error())
	}

	return this.err
}

func (this *DBHander) Close() {
	this.db.Close()
}

func (this *DBHander) InsertUser(user, email, password string) error {
	stmt, err := this.db.Prepare("INSERT user SET name=?,email=?,password=?")
	if err != nil {
		fmt.Printf("User:%s existed.", user)
		return err
	}
	defer stmt.Close()
	if res, err := stmt.Exec(user, email, password); err == nil {
		if id, err := res.LastInsertId(); err == nil {
			fmt.Println(id)
		}
	} else {
		fmt.Printf("User:%s existed.", user)
		return err
	}
	return nil
}

func (this *DBHander) SelectUserPwd(user string) (string, error) {
	cache_str := func() string {
		res, err := this.cache.GetSelect("U_" + user)
		if err == nil {
			return res
		}
		return ""
	}()

	if cache_str != "" {
		return cache_str, nil
	}

	var queryStr string
	queryStr = "SELECT password FROM user"
	queryStr += " WHERE name='" + user + "'"
	rows, err := this.db.Query(queryStr)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	var password string
	rows.Next()

	rows.Scan(&password)

	go func() {
		err := this.cache.SetSelect("U_"+user, password, G_USERINFO_EXPIRE_TIME)
		if err != nil {
			fmt.Println(err.Error())
		}
	}()

	return password, nil
}

func (this *DBHander) InsertTestFile(fileID string, storePath string, fileName string,
	size int64, fileType string, info string) (int, string) {
	stmt, err := this.db.Prepare("INSERT testfiles_0 SET FileID=?," +
		" StorePath=?,FileName=?,Size=?,FileType=?,Info=?")
	if err != nil {
		fmt.Println(err)
		return -1, err.Error()
	}
	defer stmt.Close()
	if res, err := stmt.Exec(fileID, storePath, fileName, size, fileType, info); err == nil {
		if _, err := res.LastInsertId(); err == nil {
			//fmt.Println(id)
		} else {
			fmt.Println(err)
			return -1, err.Error()
		}
	} else {
		if strings.Contains(string(err.Error()), "Duplicate") {
			getRet, already_path := this.GetTestFilePath(fileID)
			return getRet, already_path
		} else if strings.Contains(string(err.Error()), "Data too long") {
			fmt.Println(fileType)
		}

		return -1, err.Error()
		//fmt.Println(fileID)

		//fmt.Println(already_path)
	}
	return 0, ""
}

func (this *DBHander) Select(selectStr string, whereStr string, limitStr string) (string, error) {
	//	cache_str := func() string {
	//		key := whereStr
	//		if whereStr == "" {
	//			key = "ALL"
	//		}

	//		if selectStr == "COUNT(FileID)" {
	//			res, err := this.cache.GetCount(key)
	//			if err == nil {
	//				return res
	//			}
	//		} else {
	//			res, err := this.cache.GetSelect(key + "_" + limitStr)
	//			if err == nil {
	//				return res
	//			}
	//		}
	//		return ""
	//	}()

	//	if cache_str != "" {
	//		return cache_str, nil
	//	}

	var queryStr string
	queryStr = "SELECT " + selectStr + " FROM testfiles_0"
	if whereStr != "" {
		queryStr += " WHERE " + whereStr
	}
	if limitStr != "" {
		queryStr += " LIMIT " + limitStr
	}

	rows, err := this.db.Query(queryStr)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		return "", err
	}

	values := make([]sql.RawBytes, len(columns))
	scanArgs := make([]interface{}, len(values))
	for i := range values {
		scanArgs[i] = &values[i]
	}
	//	var m runtime.MemStats
	this.jsonstring = "{\"timestamp\": \"" + time.Now().Format("2006-01-02 15:04:05") + "\",\"data\":["
	allcount := 0

	//json_str := "{"
	for rows.Next() {
		this.jsonstring += "{"
		// get RawBytes from data
		err = rows.Scan(scanArgs...)
		if err != nil {
			fmt.Println(err.Error())
			return "", err
		}
		// Now do something with the data.
		// Here we just print each column as a string.
		var value string
		for i, col := range values {
			// Here we can check if the value is nil (NULL value)
			if col == nil {
				value = "NULL"
			} else {
				value = string(col)
			}
			if i == len(values)-1 {
				this.jsonstring += "\"" + columns[i] + "\":\"" + value + "\""
			} else {
				this.jsonstring += "\"" + columns[i] + "\":\"" + value + "\","
			}
		}
		this.jsonstring += "},"
		allcount++
	}

	if allcount > 0 {
		//fmt.Println(strings.LastIndex(jsonstring, ","))
		//jsonstring = SubString(string(jsonstring), 0, 203)
		bytes := []byte(this.jsonstring)
		l := strings.LastIndex(this.jsonstring, ",")
		if l > len(bytes) {
			l = len(bytes)
		}
		this.jsonstring = string(bytes[0:l])
	}
	this.jsonstring += "]}"

	//	go func() {
	//		key := whereStr
	//		if whereStr == "" {
	//			key = "ALL"
	//		}
	//		if selectStr == "COUNT(FileID)" {
	//			err := this.cache.SetCount(key, this.jsonstring)
	//			if err != nil {
	//				fmt.Println(err.Error())
	//			}
	//		} else {
	//			err := this.cache.SetSelect(key+"_"+limitStr, this.jsonstring, G_DATA_SELECT_EXPIRE_TIME)
	//			if err != nil {
	//				fmt.Println(err.Error())
	//			}
	//		}
	//	}()

	return this.jsonstring, nil
}

func (this *DBHander) GetTestFilePath(fileID string) (int, string) {
	already_rows, err1 := this.db.Query("select StorePath, FileName from testfiles_0 where FileID=?", fileID)
	if err1 != nil {
		fmt.Println(err1)
		already_rows.Close()
		return -1, ""
	}
	var storePath, fileName string
	already_rows.Next()
	already_rows.Scan(&storePath, &fileName)
	defer already_rows.Close()
	//fmt.Println(storePath)
	//fmt.Println(fileName)
	if fileName == "" {
		return -1, ""
	}

	return 0, (storePath + string(PathSeparator) + fileName)
}

func (this *DBHander) DeleteTestFile(v ...interface{}) int64 {
	argc := len(v)
	if argc == 2 {
		storePath := v[0]
		fileName := v[1]
		stmt, err := this.db.Prepare("delete from testfiles_0 where StorePath=? and FileName=?")
		if err != nil {
			fmt.Println(err)
			return -1
		}

		res, err := stmt.Exec(storePath, fileName)
		if err != nil {
			fmt.Println(err)
			return -1
		}

		affect, err := res.RowsAffected()
		if err != nil {
			fmt.Println(err)
			return -1
		}
		//		fmt.Println("***")
		//		fmt.Println(affect)
		//		fmt.Println("***")
		return affect
	} else if argc == 1 {
		fileID := v[0]
		stmt, err := this.db.Prepare("delete from testfiles_0 where FileID=?")
		if err != nil {
			fmt.Println(err)
			return -1
		}

		res, err := stmt.Exec(fileID)
		if err != nil {
			fmt.Println(err)
			return -1
		}

		affect, err := res.RowsAffected()
		if err != nil {
			fmt.Println(err)
			return -1
		}
		return affect
	}
	return -1
}

func (this *DBHander) UpdateTestFile(v ...interface{}) int {
	argc := len(v)
	switch argc {
	case 1:
		stmt, err := this.db.Prepare("update testfiles_0 set FileID=? where FileID=?")
		if err != nil {
			fmt.Println(err)
			return -1
		}
		res, err := stmt.Exec(v[1], v[1])
		if err != nil {
			fmt.Println(err)
			return -1
		}
		affect, err := res.RowsAffected()
		if err != nil {
			fmt.Println(err)
			return -1
		}
		fmt.Println(affect)
	case 2:
	case 3:
	case 4:
	case 5:
	case 6:

	}
	return -1
}

func (this *DBHander) UpdateTestFileInfo(FileIDStr string, InfoStr string) int64 {

	stmt, err := this.db.Prepare("update testfiles_0 set Info=? where FileID=?")
	if err != nil {
		fmt.Println(err)
		return -1
	}
	defer stmt.Close()
	res, err := stmt.Exec(InfoStr, FileIDStr)
	if err != nil {
		fmt.Println(err)
		return -1
	}
	affect, err := res.RowsAffected()
	if err != nil {
		fmt.Println(err)
		return -1
	}
	return affect
}

func (this *DBHander) SearchFile(md5 string, username string, filename string) (int, string, string) {
	rows, err1 := this.db.Query("select count(*) from file where user=? and filename=?", username, filename)
	if err1 != nil {
		fmt.Println(err1)
		return -1, "", ""
	}
	var cnt int
	rows.Next()
	rows.Scan(&cnt)
	fmt.Print(cnt)
	if cnt > 0 {
		fmt.Print("File already exists.\n")
		return -1, "", ""
	}

	already_rows, err1 := this.db.Query("select abspath,size from file where user=? and filename=?", username, filename)
	if err1 != nil {
		fmt.Println(err1)
		return -1, "", ""
	}
	var path_str, size_str string
	already_rows.Next()
	already_rows.Scan(&path_str, &size_str)
	fmt.Print(path_str)
	if path_str != "" {
		fmt.Print("File already exists.\n")
		return 1, path_str, size_str
	}
	return 0, "", ""
}

func (this *DBHander) Login(username string, password string) (int, string) {
	var queryStr string
	queryStr = "SELECT permission FROM user where " +
		"name='" + username + "' and password='" + password + "'"

	rows, err1 := this.db.Query(queryStr)
	if err1 != nil {
		fmt.Println(err1)
		return -1, "DBERROR:Login Query ERROR."
	}
	defer rows.Close()
	permission := -1
	for rows.Next() {
		err := rows.Scan(&permission)
		if err != nil {
			fmt.Println(err.Error())
			return -1, ""
		}
	}
	return permission, ""
}
