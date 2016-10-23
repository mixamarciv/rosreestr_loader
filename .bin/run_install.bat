::получаем curpath:
@FOR /f %%i IN ("%0") DO SET curpath=%~dp0
::задаем основные переменные окружения
@CALL "%curpath%/set_path.bat"


@del app.exe
@CLS

@echo === install ===================================================================
go get -u "github.com/nakagami/firebirdsql"
go get -u "github.com/mixamarciv/gofncstd3000"
go get -u "github.com/parnurzeal/gorequest"
go get -u "github.com/jessevdk/go-flags"

::библиотека для работы с XMLками
go get -u "github.com/jteeuwen/go-pkg-xmlx"

::библиотека для перевода struct в map[string]interface{}
go get -u github.com/fatih/structs

::библиотека для работы с html
go get -u "github.com/PuerkitoBio/goquery"

go install

@echo ==== end ======================================================================
@PAUSE
