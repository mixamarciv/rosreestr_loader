:: ===========================================================================
:: переходим в каталог запуска скрипта
::@SetLocal EnableDelayedExpansion
:: this_file_path - путь к текущему бат/bat/cmd файлу
@SET this_file_path=%~dp0

:: this_disk - диск на котором находится текущий бат/bat/cmd файл
@SET this_disk=%this_file_path:~0,2%

:: переходим в текущий каталог
@%this_disk%
CD "%this_file_path%\.."


:: ===========================================================================
:: задаем основные пути для запуска скрипта

:: пути к компилятору go
@SET GOROOT1=d:\program\go\1.7\
@SET GOROOT2=d:\program\go\1.6.2\Go\

@SET GOROOT=%GOROOT1%

:: пути к исходным кодам программы на go
@SET GOPATH=%this_file_path%\..

@SET GIT_PATH=d:\program\git
@SET PYTHON_PATH=d:\program\Python26
@SET MINGW_PATH=c:\MINGW

@SET PATH=%PYTHON_PATH%;
@SET PATH=%GOROOT1%;%GOROOT1%\bin;%PATH%;
@SET PATH=%GOROOT2%;%GOROOT2%\bin;%PATH%;
@SET PATH=%GOPATH%;%PATH%;
@SET PATH=%GIT_PATH%;%GIT_PATH%\bin;%PATH%;
@SET PATH=%MINGW_PATH%;%MINGW_PATH%\bin;%PATH%;
@SET PATH=%this_file_path%\..\bin;%PATH%;

:: ===========================================================================

