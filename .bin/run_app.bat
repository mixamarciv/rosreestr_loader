::получаем curpath:
@FOR /f %%i IN ("%0") DO SET curpath=%~dp0
::задаем основные переменные окружения
@CALL "%curpath%/set_path.bat"


@del app.exe
@CLS

@echo === build =====================================================================
go build -o app.exe

@echo ==== start ====================================================================
::app.exe
:: >> app.exe.log 2>&1

@SET start_from=220 - тут ошибки
@SET start_from=290 - тут ошибки
@SET start_from=0
@SET load_count=1
@SET load_to=1
@SET update_only=1

@SET options=--street "черн%%" --house "7%%"
@SET options=--street "мира%%" --house "25%%"
@SET options=--street "горьк%%" --house "8%%"
@SET options=--street "курат%%" --house "7%%"
@SET options=--street "мороз%%" --house "%%"
@SET options=--street "мира%%" --house "__%%"
@SET options=--street "кур%%" --house "68%%"
@SET options=--street "пол%%" --house "16%%"

::--street "кур%%" --house "68%%"

for /l %%i in (%start_from%,%load_count%,%load_to%) do (
	app.exe --load_from %%i --load_count %load_count% --update_only %update_only% %options%
)

@echo ==== end ======================================================================
@PAUSE
