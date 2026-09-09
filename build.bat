cd server
go build -o bin\vpsbackupservice.exe .\cmd\vpsbackupservice

cd ../apps/client
flutter build windows