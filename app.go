package main

import (
	//"bytes"
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	//"text/template"
	//"time"

	//xmlx "github.com/jteeuwen/go-pkg-xmlx"
	mf "github.com/mixamarciv/gofncstd3000"

	"os"
	"regexp"

	"github.com/PuerkitoBio/goquery"
	flags "github.com/jessevdk/go-flags"

	"net/http/cookiejar"
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
		//p3 <- 0
		//p2 <- 0
		p1 <- 0
	}()

	arr := make([]map[string]string, 0)
	{ //загружаем список гуидов и адресов по которым нужно загрузить kadastrn
		//LogPrint("загружаем список гуидов и адресов по которым нужно загрузить kadastrn")
		first := Itoa(opts.Load_count)
		skip := Itoa(opts.Load_from)
		query := `SELECT FIRST ` + first + ` SKIP ` + skip + `  s.strname,t.house,t.fiasguid FROM t_obj_house t 
	            		LEFT JOIN street_kladr s ON s.strcode=t.strcode
	          		WHERE 1=1
					  --AND t.house = '4'
					  --AND t.strcode = '0078'
					  AND COALESCE(t.fiasguid,'')!='' 
					  -- AND t.house = '68'
					  /*****
					  AND t.strcode != '0044'
					  AND (     COALESCE(t.kadastrn1,'') = '' 
					        OR (SELECT COUNT(*) FROM t_obj_flat f 
							    WHERE f.strcode=t.strcode AND f.house=t.house AND COALESCE(f.kadastrn1,'')=''
								) > 0
						  )
					  *****/
					ORDER BY s.name,t.house
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
			m := map[string]string{"street": mf.StrTrim(mf.StrTr(strname.get("-"), db_codepage, "utf-8")),
				"house":    mf.StrTrim(mf.StrTr(house.get("-"), db_codepage, "utf-8")),
				"fiasguid": mf.StrTrim(fiasguid.get("-"))}
			//fmt.Printf("%#v", m)

			m["macroRegionId"] = "187000000000" //http://rosreestr.ru/api/online/macro_regions   {"id":187000000000,"name":"Республика Коми"}
			m["regionId"] = "187415000000"      //http://rosreestr.ru/api/online/regions/187000000000 {"id":187415000000,"name":"Инта"}
			m["street_original"] = m["street"]
			m["n_req"] = "0"
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

	//<-p3
	//<-p2
	<-p1

}

func startload(p chan<- int, a map[string]string) {
	info := a["street"] + " " + a["house"]
	m := loaditem3(info, a, 0)
	if m["error"].(string) != "" {
		p <- 0
		LogPrint(info + " ОШИБКА: " + m["error"].(string))
		return
	}
	updatedb3(info, m)

	//LogPrint(fmt.Sprintf("%#v", m))
	//LogPrintAndExit("end test")
	p <- 1
}

func updatedb(info string, m map[string]interface{}) {
	query := "UPDATE t_obj_house SET kadastrn1=? "
	query += " WHERE fiasguid=? AND COALESCE(kadastrn1,'')='' "
	_, err := db.Exec(query, mf.StrTrim(m["kadastrn"].(string)), m["fiasguid"])
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
		query += "  AND COALESCE(kadastrn1,'')='' "
		_, err := db.Exec(query, mf.StrTrim(kadastrn), flatnumber, m["fiasguid"], mf.StrTr(m["house"].(string), "cp1251", db_codepage))
		LogPrintErrAndExit(info+" кв.["+flatnumber+"] ERROR db.Exec 2: \n"+query+"\n\n", err)
		cnt++
	}
	commit_db()
	LogPrint(fmt.Sprintf(info+" количество обновленных записей: %d", cnt))
}

func updatedb3(info string, m map[string]interface{}) {
	h := m["house_data"].(map[string]string)
	query := "UPDATE t_obj_house SET kadastrn1=?,KADASTR_PRICE=?,KADASTR_PRICE_DATE=?,KADASTR_OB_AREA=?,KADASTR_FLOOR_CNT=?, "
	query += "KADASTR_UFLOOR_CNT=?,KADASTR_WALL_INFO=?,KADASTR_BUILD_YEAR=? "
	query += " WHERE fiasguid=? AND COALESCE(kadastrn1,'')='' "
	_, err := db.Exec(query, h["kadastrn"], h["price"], h["price_date"], h["ob_area"], h["floor_cnt"],
		h["ufloor_cnt"], h["wall_info"], h["build_year"], m["fiasguid"])
	LogPrintErrAndExit(info+" ERROR db.Exec 1: \n"+query+"\n\n", err)

	flats := m["flats"].([]map[string]string)
	LogPrint(fmt.Sprintf(info+" количество квартир для обновления: %d", len(flats)))
	cnt := 0
	for _, f := range flats {
		flatnumber := f["flat"]
		if len(flatnumber) > 4 {
			msg := info + " кв.[" + flatnumber + "] ERROR неверно задан номер квартиры"
			LogPrint(msg)
			//FileAppendStr("errLoad/"+info+".log", msg+"\n")
			mf.FileAppendStr("errLoad/"+info+".log", msg+"\n"+mf.ToJsonStr(f)+"\n")
			continue
		}
		query := "UPDATE t_obj_flat f SET kadastrn1=?,KADASTR_PRICE=?,KADASTR_PRICE_DATE=?,KADASTR_OB_AREA=? "
		query += "WHERE flat=? AND strcode=(SELECT MAX(t.strcode) FROM t_obj_house t WHERE t.fiasguid=?) "
		query += "  AND house=(SELECT MAX(t.house) FROM t_obj_house t WHERE t.fiasguid=?)  "
		//query += "  AND COALESCE(kadastrn1,'')='' "
		_, err := db.Exec(query, m["kadastrn"], m["price"], m["price_date"], m["ob_area"],
			flatnumber, m["fiasguid"], m["fiasguid"])
		LogPrintErrAndExit(info+" кв.["+flatnumber+"] ERROR db.Exec 2: \n"+query+"\n\n", err)

		cnt++
	}

	commit_db()
	LogPrint(fmt.Sprintf(info+" количество обновленных записей: %d", cnt))
}

func loaditem2(info string, m map[string]string, n_req int) map[string]interface{} {
	url := "http://rosreestr.ru/api/online/address/fir_objects"
	//LogPrint("URL:> " + url)

	n := map[string]string{}
	n["macroRegionId"] = m["macroRegionId"] //http://rosreestr.ru/api/online/macro_regions   {"id":187000000000,"name":"Республика Коми"}
	n["regionId"] = m["regionId"]           //http://rosreestr.ru/api/online/regions/187000000000 {"id":187415000000,"name":"Инта"}
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
		matched, _ := regexp.MatchString("[AaАаБбBbВв]", m["house"])
		if matched && n_req < 4 {
			matched2, _ := regexp.MatchString("[AaАа]", m["house"])
			if matched2 {
				t := map[int]string{0: "A", 1: "a", 2: "А", 3: "а"}
				m["house"] = mf.StrRegexpReplace(m["house"], "[AaАа]", t[n_req])
				return loaditem2(info, m, n_req+1)
			}
			matched2, _ = regexp.MatchString("[Бб]", m["house"])
			if matched2 && n_req < 2 {
				t := map[int]string{0: "Б", 1: "б"}
				m["house"] = mf.StrRegexpReplace(m["house"], "[Бб]", t[n_req])
				return loaditem2(info, m, n_req+1)
			}
			matched2, _ = regexp.MatchString("[BbВв]", m["house"])
			if matched2 {
				t := map[int]string{0: "B", 1: "b", 2: "В", 3: "в"}
				m["house"] = mf.StrRegexpReplace(m["house"], "[BbБб]", t[n_req])
				return loaditem2(info, m, n_req+1)
			}
		}

		matched3, _ := regexp.MatchString("Абезь", m["street_original"])
		//LogPrint(fmt.Sprintf("%v %v", matched3, err))
		//LogPrintErrAndExit(info+" ERROR regexp: \nregexp.MatchString(\"Абезь\", \""+m["street"]+"\")\n\n", err)
		if matched3 {
			m["street"] = mf.StrTrim(mf.StrRegexpReplace(m["street"], "Абезь", ""))
			if m["regionId"] == "187415000000" {
				m["regionId"] = "187415802001" // <option value="187415802001">Абезь (Поселок сельского типа)</option>
				return loaditem2(info, m, 0)
			}
			if m["regionId"] == "187415802001" {
				m["regionId"] = "187415802002" // <option value="187415802002">Абезь (Деревня)</option>
				return loaditem2(info, m, 0)
			}
			LogPrint("== [ERROR:]==================================================================")
			LogPrint(jsonStr)
			LogPrint("== [/ERROR]==================================================================")
		}

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
		//LogPrint("=============================================================================")
		//LogPrint(jsonStr)
		//LogPrint(mf.ToJsonStr(r))
		LogPrint("=============================================================================")
		ret := map[string]interface{}{"street": m["street"], "house": m["house"], "fiasguid": m["fiasguid"], "kadastrn": ""}
		ret["error"] = ""
		ret["kadastrn"] = ""
		flats := make(map[string]string, 0)
		for _, flat := range r["objects"].([]interface{}) {
			f := flat.(map[string]interface{})
			apartment := f["apartment"].(string)
			kadastrn := f["objectCn"].(string)
			if apartment == "null" && kadastrn != "null" {
				LogPrint("---------------------------------------------------------------------")
				//для проверки можно использовать http://rosreestr.ru/api/online/fir_object/::objectId::
				//но многих данных через апи всеравно нет, хотя их можно получить через https://rosreestr.ru/wps/portal/online_request
				//поэтому загружаем все через loaditem3
				delete(f, "house")
				delete(f, "apartment")
				delete(f, "subjectId")
				delete(f, "objectCon")
				delete(f, "regionKey")
				delete(f, "street")
				delete(f, "okato")
				delete(f, "objectType")
				delete(f, "regionId")
				delete(f, "settlementId")
				delete(f, "regionId")
				s := mf.ToJsonStr(f)
				s = strings.Replace(s, "\",\"", "\",\n\"", -1)
				LogPrint(s)
				if ret["kadastrn"] == "" {
					ret["kadastrn"] = kadastrn
				}
				continue
			}
			flats[apartment] = kadastrn
		}
		ret["flats"] = flats
		//LogPrint("=============================================================================")
		//LogPrint(mf.ToJsonStr(ret))
		//LogPrint("=============================================================================")
		return ret
	}
}

type urllist struct {
	a []map[string]string
}

func (p *urllist) add(m map[string]string) {
	p.a = append(p.a, m)
}

var url_host string = "https://rosreestr.ru"

func loaditem3(info string, m map[string]string, n_req int) map[string]interface{} {
	LogPrint("\n" + info + " load")
	ret := make(map[string]interface{})

	url := url_host + "/wps/portal/online_request"
	doc, err := goquery.NewDocument(url)
	if err != nil {
		LogPrint(Fmts("%#v", err))
		LogPrintAndExit("request send error: \n url: " + url + "\n\n")
	}

	sel := doc.Find("[method=post]")
	if len(sel.Nodes) == 0 {
		ret["error"] = "not found form[name=search_action]"
		return ret
	}

	action, _ := sel.Attr("action")
	//LogPrint("action: " + action)

	//------------------------------------------------------
	//отправляем запрос с сортировкой по адресам(&sortField=ADDRESS_NOTE&sortOrder=asc)
	requrl := url_host + action + "&sortField=ADDRESS_NOTE&sortOrder=asc"
	body, d := SendHttpRequestPOST_loadlist(requrl, m)

	//LogPrint("=============================================================================")
	//mf.FileWrite("test/"+info+".url", []byte(requrl))
	//mf.FileWrite("test/"+info+".res", res)
	//LogPrintAndExit(fmt.Sprintf("%#v", string(res)))
	//LogPrint("=============================================================================")

	//-------------------------------------------------------
	//получаем список ссылок со всех страницы результатов
	alist := new(urllist)
	loadpagelinks(info, 1, body, alist, d)

	//mf.FileWrite("test/"+info+".a", []byte(fmt.Sprintf("loaded list: %d\n\n%#v", len(alist.a), alist.a)))
	LogPrint(fmt.Sprintf("loaded list: %d", len(alist.a)))

	//--------------------------------------------------------
	//загружаем и парсим каждую страницу в map[string]string
	ret = map[string]interface{}{"street": m["street"], "house": m["house"], "fiasguid": m["fiasguid"], "error": ""}

	data_flats := make([]map[string]string, 0)
	data_houses := make([]map[string]string, 0)

	//может быть несколько кадастровых номеров для одного и того же здания, выбираем первый с максимальной площадью здания
	house_max_ob_area_zd := -1
	house_ok_i_zd := -1

	//может быть несколько кадастровых номеров для одного и тоже здания, без указания параметра что это здание, выбираем с макс. площадью
	house_max_ob_area_hz := -1
	house_ok_i_hz := -1

	for i, m := range alist.a {
		if i > 10 {
			//break
		}

		adres := m["name"]
		url := url_host + m["url"]
		t := loadkadastrinfo(info+" "+adres, url, d)

		if t["type"] == "квартира" {
			data_flats = append(data_flats, t)
			LogPrint(fmt.Sprintf(info+" загружены данные по квартире: \"%s\" "+adres, t["flat"]))
		} else {
			data_houses = append(data_houses, t)
			//LogPrint(fmt.Sprintf(adres+" %v", t))

			v, _ := strconv.Atoi(t["ob_area"])
			if t["type"] == "здание" {
				LogPrint(fmt.Sprintf(info + " загружены данные по зданию: " + adres))
				if house_ok_i_zd < 0 {
					house_ok_i_zd = len(data_houses) - 1
				}
				if house_max_ob_area_zd < v {
					house_ok_i_zd = len(data_houses) - 1
					house_max_ob_area_zd = v
				}
			} else {
				LogPrint(fmt.Sprintf(info + " загружены данные по " + t["type"] + ": " + adres))
				if house_max_ob_area_hz < v {
					house_ok_i_hz = len(data_houses) - 1
					house_max_ob_area_hz = v
				}
			}
		}
	}

	ret["flats"] = data_flats

	if house_ok_i_zd >= 0 {
		ret["house_data"] = data_houses[house_ok_i_zd]
	} else {
		ret["house_data"] = data_houses[house_ok_i_hz]
	}

	//LogPrint("=============================================================================")
	//LogPrint(mf.ToJsonStr(ret["house_data"]))
	//LogPrint("=============================================================================")

	//LogPrintAndExit("конец")
	return ret

	/**********************
			ret := map[string]interface{}{"street": m["street"], "house": m["house"], "fiasguid": m["fiasguid"], "kadastrn": ""}
			ret["error"] = ""
			ret["kadastrn"] = ""
			flats := make(map[string]string, 0)
			for _, flat := range r["objects"].([]interface{}) {
				f := flat.(map[string]interface{})
				apartment := f["apartment"].(string)
				kadastrn := f["objectCn"].(string)
				if apartment == "null" && kadastrn != "null" {
					LogPrint("---------------------------------------------------------------------")
					//для проверки можно использовать http://rosreestr.ru/api/online/fir_object/::objectId::
					//но многих данных через апи всеравно нет, хотя их можно получить через https://rosreestr.ru/wps/portal/online_request
					//поэтому загружаем все через loaditem3
					delete(f, "house")
					delete(f, "apartment")
					delete(f, "subjectId")
					delete(f, "objectCon")
					delete(f, "regionKey")
					delete(f, "street")
					delete(f, "okato")
					delete(f, "objectType")
					delete(f, "regionId")
					delete(f, "settlementId")
					delete(f, "regionId")
					s := mf.ToJsonStr(f)
					s = strings.Replace(s, "\",\"", "\",\n\"", -1)
					LogPrint(s)
					if ret["kadastrn"] == "" {
						ret["kadastrn"] = kadastrn
					}
					continue
				}
				flats[apartment] = kadastrn
	**********************/

	//LogPrint("not found form[name=search_action]")

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

//отправляем запрос на список адресов по дому
func SendHttpRequestPOST_loadlist(urlStr string, m map[string]string) ([]byte, map[string]interface{}) {
	data := url.Values{}

	//subject_id: <option value="187000000000">Республика Коми</option>
	//data.Set("subject_id", "187000000000")

	/**************************
	settlement_id:
	<option value="187415802001">Абезь (Поселок сельского типа)</option>
	<option value="187415802002">Абезь (Деревня)</option>
	<option value="187415803002">Адзьва (Деревня)</option>
	<option value="187415803001">Адзьвавом (Село)</option>
	<option value="187415562000">Верхняя Инта (Город)</option>
	<option value="187415802003">Епа (Деревня)</option>
	<option value="287415000000">ИНТА (Город)</option>
	<option value="187415573000">Кожым (Город)</option>
	<option value="187415573001">Кожымвом (Деревня)</option>
	<option value="187415573002">Комаю (Поселок сельского типа)</option>
	<option value="187415810002">Костюк (Поселок сельского типа)</option>
	<option value="187415807001">Косьювом (Село)</option>
	<option value="187415807002">Кочмес (Поселок сельского типа)</option>
	<option value="187415562002">Кочмес (Поселок сельского типа)</option>
	<option value="187415573003">Лазурный (Поселок сельского типа)</option>
	<option value="187415810001">Петрунь (Село)</option>
	<option value="187415802000">поселок сельского типа Абезь</option>
	<option value="187415810003">Роговая (Деревня)</option>
	<option value="187415803000">село Адзьвавом</option>
	<option value="187415807000">село Косьювом</option>
	<option value="187415810000">село Петрунь</option>
	<option value="187415802004">Тошпи (Деревня)</option>
	<option value="187415802005">Уса (Поселок сельского типа)</option>
	<option value="187415802006">Фион (Поселок сельского типа)</option>
	<option value="187415000002">Юсьтыдор (Поселок сельского типа)</option>
	<option value="187415807003">Ягъёль (Деревня)</option>
	<option value="187415802007">Ярпияг (Деревня)</option>
	***************************/
	//data.Add("settlement_id", "287415000000")

	//--------------------------------------------------------------------------
	/***************************
	//обязательно должны быть заданы все следующие поля(могут быть пустые но должны быть):
	search_action:true
	subject:
	region:
	settlement:
	cad_num:
	start_position:59
	obj_num:
	old_number:
	search_type:ADDRESS
	src_object:1
	subject_id:187000000000
	region_id:187415000000
	settlement_type:-1
	settlement_id:-1
	street_type:str0
	street:Мира
	house:69
	building:
	structure:
	apartment:
	r_subject_id:101000000000
	right_reg:
	encumbrance_reg:
	****************************/
	data.Set("search_action", "true")
	data.Add("subject", "")
	data.Add("region", "")
	data.Add("settlement", "")
	data.Add("start_position", "59")
	data.Add("obj_num", "")
	data.Add("old_number", "")
	data.Add("search_type", "ADDRESS")
	data.Add("src_object", "1")
	data.Add("subject_id", "187000000000")
	data.Add("region_id", "187415000000")
	data.Add("settlement_type", "-1")
	data.Add("settlement_id", "-1")
	data.Add("street_type", "str0")
	data.Add("street", m["street"])
	data.Add("house", m["house"])
	data.Add("building", "")
	data.Add("structure", "")
	data.Add("apartment", "")
	data.Add("r_subject_id", "101000000000")
	data.Add("right_reg", "")
	data.Add("encumbrance_reg", "")
	//--------------------------------------------------------------------------

	data_enc := data.Encode()

	cookieJar, _ := cookiejar.New(nil)
	client := &http.Client{
		Jar: cookieJar,
	}

	r, err := http.NewRequest("POST", urlStr, bytes.NewBufferString(data_enc))
	LogPrintErrAndExit("http.NewRequest error: \n urlStr: "+urlStr+"\n\n", err)
	//r.Header.Add("Authorization", "auth_token=\"XXXXXXX\"")
	r.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Add("Content-Length", strconv.Itoa(len(data_enc)))

	resp, err := client.Do(r)
	LogPrintErrAndExit("http.NewRequest.client.Do error: \n urlStr: "+urlStr+"\n\n", err)

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	LogPrintErrAndExit("http.resp.Body Read error: \n urlStr: "+urlStr+"\n\n", err)

	d := map[string]interface{}{"Referer": resp.Header.Get("Referrer")}
	if d["Referer"].(string) == "" {
		d["Referer"] = urlStr
	}
	d["cookie"] = cookieJar //куки нужны для просмотра след.страниц
	return body, d
}

//получаем просто результат гет запроса:
func SendHttpRequestGET(info string, urlStr string, d map[string]interface{}) []byte {
	//cookieJar, _ := cookiejar.New(nil)
	time.Sleep(time.Second * 1)
	client := &http.Client{
		Jar: d["cookie"].(http.CookieJar),
	}

	r, err := http.NewRequest("GET", urlStr, nil)
	LogPrintErrAndExit("SendHttpRequestGET: http.NewRequest error: \n "+info+" \n urlStr: "+urlStr+"\n\n", err)
	//r.Header.Add("Authorization", "auth_token=\"XXXXXXX\"")
	r.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	//LogPrint("Referer: " + d["Referer"].(string))
	r.Header.Add("Referer", d["Referer"].(string))

	//r.Header.Add("Content-Length", strconv.Itoa(len(data_enc)))

	resp, err := client.Do(r)
	LogPrintErrAndExit("SendHttpRequestGET: http.NewRequest.client.Do error: \n "+info+" \n urlStr: "+urlStr+"\n\n", err)

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	LogPrintErrAndExit("SendHttpRequestGET: http.resp.Body Read error: \n "+info+" \n urlStr: "+urlStr+"\n\n", err)

	//mf.FileWrite("test/"+info+".res", body)

	referer := resp.Header.Get("Referrer")
	if referer == "" {
		referer = urlStr
	}
	d["Referer"] = urlStr
	return body
}

//получаем список ссылок по адресам со всех страниц рекурсивно загружая эти страницы
func loadpagelinks(info string, page int, body []byte, alist *urllist, d map[string]interface{}) {
	//mf.FileWrite("test/"+info+".res"+strconv.Itoa(page), body)
	buff := bytes.NewBuffer(body)
	doc, err := goquery.NewDocumentFromReader(buff)
	LogPrintErrAndExit("goquery.NewDocumentFromReader error: \n"+info+" page:"+strconv.Itoa(page), err)

	//--------------------------------------------------------------------------
	//ищем блок с ссылками на адреса:
	table := doc.Find(".brdw1111")
	if table.Length() == 0 {
		LogPrintAndExit("not found elements for selector '.brdw1111'  " + info + " page:" + strconv.Itoa(page))
	}

	a := table.Find(".td").Find("a")
	if a.Length() == 0 {
		LogPrintAndExit("not found elements for selector  '.brdw1111'>'.td'>'.a'  " + info + " page:" + strconv.Itoa(page))
	}

	//LogPrint(fmt.Sprintf("t:%d    td:%d    a:%d", table.Length(), td.Length(), a.Length()))
	//сохраняем ссылки на адреса в alist:
	for i := 0; i < a.Length(); i++ {
		ai := a.Eq(i)
		if ai.Length() == 0 {
			LogPrint(fmt.Sprintf("skip1 i:%d a:%s", i, ai.Text()))
			continue
		}
		text := mf.StrTrim(ai.Text())
		href, _ := ai.Attr("href")
		b := strings.Index(href, "object_data_id=")
		if b == -1 {
			LogPrint(fmt.Sprintf("skip2 i:%d a:%s", i, text))
			continue
		}
		m := map[string]string{"url": href, "name": text}

		alist.add(m)
		LogPrint(fmt.Sprintf("ok i:%d a:%s", i, text))
		//mf.FileWrite("test/"+text+".a", []byte(href))
	}
	//LogPrint(fmt.Sprintf("loaded list: %d\n\n", len(alist.a)))

	//--------------------------------------------------------------------------
	//ищем ссылку на следующую страницу:
	a2 := table.Find("table").Find("table").Find("a")
	if a2.Length() == 0 {
		//кнопок переключения страниц просто нет
		return
		//LogPrintAndExit("not found elements for selector  '.brdw1111'>'table'>'table'>'a'  " + info + " page:" + strconv.Itoa(page))
	}

	NextPagestr := strconv.Itoa(page + 1)
	needHref := ""

	for i := 0; i < a2.Length(); i++ {
		ai := a2.Eq(i)
		if ai.Length() == 0 {
			LogPrint(fmt.Sprintf("skip1 i:%d a:%s", i, ai.Text()))
			continue
		}
		text := mf.StrTrim(ai.Text())
		href, _ := ai.Attr("href")
		b := strings.Index(href, "_online_request_search_page=")
		if b == -1 {
			LogPrint(fmt.Sprintf("skip2 i:%d a:%s", i, text))
			continue
		}
		if text == NextPagestr {
			needHref = url_host + href
			break
		}
		//LogPrint(fmt.Sprintf("page: %s\n\n", text))
	}

	//если ссылку на след. страницу не нашли то выходим из функции
	if needHref == "" {
		return
	}

	//загружаем следующую страницу
	nextpagebody := SendHttpRequestGET(info+"_page"+strconv.Itoa(page+1), needHref, d)

	loadpagelinks(info, page+1, nextpagebody, alist, d)
}

//загружаем данные по отдельной квартире-дому и сохраняем в map[string]interface{}
func loadkadastrinfo(info string, urlStr string, d map[string]interface{}) map[string]string {
	body := SendHttpRequestGET(info, urlStr, d)
	buff := bytes.NewBuffer(body)
	doc, err := goquery.NewDocumentFromReader(buff)
	LogPrintErrAndExit("loadkadastrinfo: goquery.NewDocumentFromReader error: \n"+info+":\n", err)

	t := doc.Find(".brdw1010").Find("table").Eq(0)
	if t.Length() == 0 {
		mf.FileWrite("test/"+info+".ERR", body)
		LogPrintAndExit("not found elements for selector '.brdw1010'>'table'  " + info)
	}

	ret := make(map[string]string)
	ret["kadastrn"] = findTrFieldByText(info, "Кадастровый номер", ".*", t, "-", true)
	ret["ob_area"] = strings.Replace(findTrFieldByText(info, "Площадь ОКС", "^[\\d\\.,]+$", t, "0", false), ",", ".", -1)
	ret["type"] = strings.ToLower(findTrFieldByText(info, "(ОКС) Тип:", ".*", t, "-", false))
	ret["price"] = strings.Replace(findTrFieldByText(info, "Кадастровая стоимость", "^[\\d\\.,]+$", t, "-", false), ",", ".", -1)
	ret["price_date"] = findTrFieldByText(info, "Дата утверждения стоимости", "^[\\d\\.-]+$", t, "-", false)

	ret["floor_cnt"] = findTrFieldByText(info, "Этажность", "^\\d+$", t, "0", false)
	ret["ufloor_cnt"] = findTrFieldByText(info, "одземн", "^\\d$", t, "0", false)
	ret["wall_info"] = findTrFieldByText(info, "Материал стен", ".*", t, "0", false)
	ret["build_year"] = findTrFieldByText(info, "Ввод в эксплуатацию", "^[\\d\\.-]+$", t, "0", false)

	ret["flat"] = "-"
	if ret["type"] == "квартира" {
		flat := findTrFieldByText(info, "Адрес (местоположение)", ".*", t, "-", false)
		ret["flat"] = clearTextAddress(flat)
	}

	//LogPrint(fmt.Sprintf(info+" %v", ret))

	//Кадастровый номер
	//LogPrintAndExit(t.Text())
	return ret
}

//возвращает текст из соседнего td поля
func findTrFieldByText(info, text, testRegexp string, s *goquery.Selection, defaultval string, isrequired bool) string {
	a := s.Find("tr:contains(\"" + text + "\")")
	if a.Length() == 0 {
		if isrequired {
			body, _ := s.Html()
			mf.FileWrite("test/"+info+".ERR", []byte(body))
			LogPrintAndExit("not found elements for selector '.brdw1010'>'table'>tr:contains(\"" + text + "\")  " + info)
		}
		return defaultval
	}
	a = a.Find("td")
	if a.Length() < 2 {
		if isrequired {
			body, _ := s.Html()
			mf.FileWrite("test/"+info+".ERR", []byte(body))
			LogPrintAndExit("not found elements for selector '.brdw1010'>'table'>tr:contains(\"" + text + "\")>td" + info)
		}
		return defaultval
	}
	t := mf.StrTrim(a.Eq(1).Text())
	if t == "" {
		if isrequired {
			body, _ := s.Html()
			mf.FileWrite("test/"+info+".ERR", []byte(body))
			LogPrintAndExit("no text in element for selector '.brdw1010'>'table'>tr:contains(\"" + text + "\")>td" + info)
		}
		return defaultval
	}
	if mf.StrRegexpMatch(testRegexp, t) == false {
		if isrequired {
			body, _ := s.Html()
			mf.FileWrite("test/"+info+".ERR", []byte(body))
			LogPrintAndExit("not match text(\"" + t + "\") in element for selector '.brdw1010'>'table'>tr:contains(\"" + text + "\")>td" + info)
		}
		return defaultval
	}
	return t
}

//возвращает текст после последней запятой с удаленными лишними символами "кв." "кв" и т.д.
func clearTextAddress(text string) string {
	s := strings.ToLower(mf.StrTrim(text))
	i := strings.LastIndex(s, ",")
	if i < 0 {
		return s
	}
	s = s[i+1:]

	s = strings.Replace(s, ".", "", -1)
	s = strings.Replace(s, "-", "", -1)
	s = strings.Replace(s, "помещение", "", -1)
	s = strings.Replace(s, "помещен", "", -1)
	s = strings.Replace(s, "помещ", "", -1)
	s = strings.Replace(s, "пом", "", -1)
	s = strings.Replace(s, "п", "", -1)
	s = strings.Replace(s, "номер", "", -1)
	s = strings.Replace(s, "ном", "", -1)
	s = strings.Replace(s, "н", "", -1)
	s = strings.Replace(s, "квартира", "", -1)
	s = strings.Replace(s, "кварт", "", -1)
	s = strings.Replace(s, "квар", "", -1)
	s = strings.Replace(s, "квра", "", -1)
	s = strings.Replace(s, "кв", "", -1)

	s = mf.StrTrim(s)
	if s != "" && s[0:1] == "0" {
		s = s[1:]
	}
	return s
}

func commit_db() {
	query := `commit`
	_, err := db.Exec(query)
	LogPrintErrAndExit("ОШИБКА выполнения запроса: \n"+query+"\n\n", err)
}
