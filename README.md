## API получения профиля по JWT

Запускаем приложение командами из Makefile.

Скачиваем образ докера postgres и запускаем БД:
```bash
$ make docker-build
```
Если контейнер postgres уже есть, запускаем этой командой его:
```bash
$ make docker-run
```
Собираем приложение:
```bash
$ make build
```
Запускаем приложение:
```bash
$ make run
```
Тесты:
```bash
$ make test
```


В апи есть 5 роутов: 

/account - метод POST, создаем аккаунт, отправляем тело json: 
```bash
{
  "firstName": "Name",
  "lastName": "Last Name",
  "password": "pass"
}
```
/login - метод POST, авторизация, получаем JWT токен, отправляем json: 
```bash
{
  "number" : 398695, - счет аккаунта
  "password" : "pass"
}
```
/account - метод GET, список аккаунтов

/account/{id} - метод GET, доступ к своему аккаунту, в heders добавляем значение jwt токена с ключем x-jwt-token. Проверка токена через middleware с помощью декораторов 

/account/{id} - метод DELETE, удаляем аккаунт. TODO: нужно обернуть в декоратор

/transfer - метод POST, перевод баланса другому пользователю. TODO: еще не реализовано. 