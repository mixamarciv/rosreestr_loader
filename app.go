package main

import (
	//"bytes"
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	//"text/template"
	//"time"

	//xmlx "github.com/jteeuwen/go-pkg-xmlx"
	mf "github.com/mixamarciv/gofncstd3000"

	flags "github.com/jessevdk/go-flags"

	"os"
	//s "strings"
	"github.com/PuerkitoBio/goquery"
)

var Fmts = fmt.Sprintf
var Print = fmt.Print
var Itoa = strconv.Itoa

func main() {
	Initdb()

	var opts struct {
		Load_from   int `long:"load_from" description:"start load id from"`
		Load_count  int `long:"load_count" description:"count id load"`
		Update_only int `long:"update_only" description:"update only" default:"0"`
	}
	_, err := flags.ParseArgs(&opts, os.Args)
	LogPrintErrAndExit("ошибка разбора параметров", err)

	p1 := make(chan int)
	p2 := make(chan int)
	p3 := make(chan int)
	go func() {
		p3 <- 0
		p2 <- 0
		p1 <- 0
	}()

	arr := make([]map[string]string, 0)
	{ //загружаем список гуидов и адресов по которым нужно загрузить kadastrn
		//LogPrint("загружаем список гуидов и адресов по которым нужно загрузить kadastrn")
		first := Itoa(opts.Load_count)
		skip := Itoa(opts.Load_from)
		query := `SELECT FIRST ` + first + ` SKIP ` + skip + `  s.strname,t.house,t.fiasguid FROM t_obj_house t 
	            		LEFT JOIN street_kladr s ON s.strcode=t.strcode
	          		WHERE COALESCE(t.fiasguid,'')!='' ORDER BY s.name,t.house
			 	`
		rows, err := db.Query(query)
		LogPrintErrAndExit("ERROR db.Query: \n"+query+"\n\n", err)
		cnt := 0
		for rows.Next() {
			cnt++
			var strname, house, fiasguid NullString

			if err := rows.Scan(&strname, &house, &fiasguid); err != nil {
				LogPrintErrAndExit("ERROR rows.Scan: \n"+query+"\n\n", err)
			}
			m := map[string]string{"street": mf.StrTr(strname.get("-"), db_codepage, "utf-8"),
				"house":    mf.StrTr(house.get("-"), db_codepage, "utf-8"),
				"fiasguid": fiasguid.get("-")}
			//fmt.Printf("%#v", m)
			arr = append(arr, m)
		}
		LogPrint("\n\nзагружаем данные по " + Itoa(cnt) + " домам(у)")
	}

	{
		LogPrint("загрузка данных с сайта")
		for i := 0; i < len(arr); i++ {
			a := arr[i]
			select {
			case <-p1:
				go startload(p1, a)
			case <-p2:
				go startload(p2, a)
			case <-p3:
				go startload(p3, a)
			}
		}
	}

	<-p3
	<-p2
	<-p1

}

func startload(p chan<- int, a map[string]string) {
	info := a["street"] + " " + a["house"]
	m := loaditem2(info, a)
	if m["error"].(string) != "" {
		p <- 0
		LogPrint(info + " ОШИБКА: " + m["error"].(string))
		return
	}
	updatedb(info, m)

	//LogPrint(fmt.Sprintf("%#v", m))
	//LogPrintAndExit("end test")
	p <- 1
}

func updatedb(info string, m map[string]interface{}) {
	query := "UPDATE t_obj_house SET kadastrn1=? "
	query += " WHERE fiasguid=? "
	_, err := db.Exec(query, m["kadastrn"], m["fiasguid"])
	LogPrintErrAndExit(info+" ERROR db.Exec 1: \n"+query+"\n\n", err)

	flats := m["flats"].(map[string]string)
	cnt := 0
	for flatnumber, kadastrn := range flats {
		if len(flatnumber) > 4 {
			LogPrint(info + " кв.[" + flatnumber + "] ERROR неверно задан номер квартиры")
			continue
		}
		query := "UPDATE t_obj_flat f SET kadastrn1=? "
		query += "WHERE flat=? AND strcode=(SELECT MAX(t.strcode) FROM t_obj_house t WHERE t.fiasguid=?) AND house=? "
		_, err := db.Exec(query, kadastrn, flatnumber, m["fiasguid"], mf.StrTr(m["house"].(string), "cp1251", db_codepage))
		LogPrintErrAndExit(info+" кв.["+flatnumber+"] ERROR db.Exec 2: \n"+query+"\n\n", err)
		cnt++
	}
	commit_db()
	LogPrint(fmt.Sprintf(info+" количество обновленных записей: %d", cnt))
}

func loaditem2(info string, m map[string]string) map[string]interface{} {
	url := "http://rosreestr.ru/api/online/address/fir_objects"
	//LogPrint("URL:> " + url)

	n := map[string]string{}
	n["macroRegionId"] = "187000000000" //http://rosreestr.ru/api/online/macro_regions   {"id":187000000000,"name":"Республика Коми"}
	n["regionId"] = "187415000000"      //http://rosreestr.ru/api/online/regions/187000000000 {"id":187415000000,"name":"Инта"}
	n["street"] = m["street"]
	n["house"] = m["house"]

	var jsonStr = mf.ToJsonStr(n)
	//LogPrint("jsonStr:> " + jsonStr)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer([]byte(jsonStr)))
	req.Header.Set("X-Custom-Header", "myvalue")
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	LogPrintErrAndExit(info+" request send error client.Do: \n url: "+url+"\n\n", err)
	defer resp.Body.Close()

	//LogPrint(fmt.Sprintf("response Status:", resp.Status))
	//LogPrint(fmt.Sprintf("response Headers:", resp.Header))
	body, _ := ioutil.ReadAll(resp.Body)
	//LogPrint("response Body: \n" + string(body))

	text := string(body)
	if text == "" {
		return map[string]interface{}{"error": "404 not found"}
	}

	text = mf.StrRegexpReplace(text, "\":null([,\\}\\{\\]\\[])", "\":\"null\"$1")
	text = mf.StrRegexpReplace(text, "\":(\\d+)([,\\}\\{\\]\\[])", "\":\"$1\"$2")
	text = "{\"objects\":" + text + "}"

	//LogPrint("=============================================================================")
	//LogPrint(text)
	//LogPrint("=============================================================================")

	r := mf.FromJsonStr([]byte(text))
	if _, err := r["error"]; err {
		return r
	}

	{ //выбираем нужные данные в map[string]interface{} и возвращаем их
		ret := map[string]interface{}{"street": m["street"], "house": m["house"], "fiasguid": m["fiasguid"]}
		ret["error"] = ""
		ret["kadastrn"] = ""
		flats := make(map[string]string, 0)
		for _, flat := range r["objects"].([]interface{}) {
			f := flat.(map[string]interface{})
			apartment := f["apartment"].(string)
			kadastrn := f["nobjectCn"].(string)
			if apartment == "null" {
				ret["kadastrn"] = kadastrn
				continue
			}
			flats[apartment] = kadastrn
		}
		ret["flats"] = flats
		return ret
	}
}

func loaditem3(m map[string]string) {
	LogPrint("load " + m["street"] + " " + m["house"] + "  [" + m["fiasguid"] + "]")

	url := "https://rosreestr.ru/wps/portal/online_request"
	doc, err := goquery.NewDocument(url)
	if err != nil {
		LogPrint(Fmts("%#v", err))
		LogPrintAndExit("request send error: \n url: " + url + "\n\n")
	}

	sel := doc.Find("[method=post]")
	if len(sel.Nodes) == 0 {
		LogPrint("not found form[name=search_action]")
		return
	}

	action, _ := sel.Attr("action")
	LogPrint("action: " + action)

	//req, err := http.NewRequest("POST", url, strings.NewReader(form.Encode()))

	LogPrint("not found form[name=search_action]")
	/********
		skip := 0 //флаг для загрузки ордеров с другого ресурса
		sels := doc.Find(".price_check tr")
		sels_type := "eve-marketdata.com"
		if len(sels.Nodes) < 2 {
			//LogPrint("skip " + sid + ": not found sell_orders")
			skip++
		}
	*******/
	/***************
	url := "https://rosreestr.ru/wps/portal/online_request"
	doc, err := goquery.NewDocument(url)
	if err != nil {
		LogPrint(Fmts("%#v", err))
		LogPrintAndExit("request send error: \n url: " + url + "\n\n")
	}

	sel := doc.Find("h1")
	if len(sel.Nodes) == 0 {
		LogPrint("skip " + sid + ": not found h1")
		return
	}
	name := Trim(sel.Text())

	skip := 0 //флаг для загрузки ордеров с другого ресурса
	sels := doc.Find(".price_check tr")
	sels_type := "eve-marketdata.com"
	if len(sels.Nodes) < 2 {
		//LogPrint("skip " + sid + ": not found sell_orders")
		skip++
	}

	LogPrint("load " + sid + ": " + name + ";  " + Itoa(len(sels.Nodes)) + " / " + Itoa(len(selb.Nodes)) + " " + info)
	****************/
}

func commit_db() {
	query := `commit`
	_, err := db.Exec(query)
	LogPrintErrAndExit("ОШИБКА выполнения запроса: \n"+query+"\n\n", err)
}
