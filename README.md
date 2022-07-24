# thumbnails

This project creates thumbnails of jpg files replicating the dir structure of the original (source) directory tree.

It also runs as a web server returning full size images ans thumbnail images on demand.

## Usage (not a Server)

``` bash
thumbnails source-path dest-path size=200 mask=%YYYY_%MM_%DD_%h_%m_%s_%n.%x noclobber=true
```

| Value | Desc | Optional |
| ----------- | ----------- | ----------- |
| source-path | is the root directory containing the original pictures (.jpg or .png) | required|
| dest-path | is the root directory that will contain the thumbnail pictures (.jpg) | required|
| size=N | is the minimum width or height for the thumbnail depending on the aspect ratio | optional = 200 |
| mask=M | is the format of the file name of the thumbnail created | optional = See below |
| noclobber=T | if 'true' then existing thumbnails will not be overwritten | optional = false |
| verbose | if present then event data is logged | optional = not verbose |
| serverport=P | if present runs as a server on that port | optional = not a server |
| help | will display the help text | optional = false |

The dest-path is assumed to be empty. All required directories will be created.

If the picture is taller than wide the thumbnail will be \<size\> wide.
If the picture is wider than tall the thumbnail will be \<size\> high.

The thumbnails will be oriented according to the exif --> orientation field in the jpg meta data.

If the exif --> orientation cannot be derived then the it is assumed to be 1 (rotate 0 degrees)

## Mask

The default mask is '%YYYY_%MM_%DD_%h_%m_%s_%n.%x'

| Value | Desc |
| ----------- | ----------- |
| %YYYY | is a 4 digit year |
| %MM | is a 2 digit month  |
| %DD | is a 2 digit day of month |
| %h | is a 2 digit hour in 24 hour format |
| %m | is a 2 digit minute |
| %s | is a 2 digit second |
| %n | is the name of the original file without the suffix (.jpg) |
| %x | if always 'jpg' which is the format of the thumbnail file. |

The time used is derived from the meta data in the original image.

If that is not available then the file name is parsed for a time.

If that fails then the file system 'modified' time is used.

As a last resort the current date time is used.

## Usage as a Server

The server has a json configuration file. Pass it's location in using 'serverconfig=' parameter.

``` json
{
    "resources": {
        "users": {
            "user1": {
                "name": "User 1 name",
                "dir1": "files1",
                "dir2": "files2/docs"
            },
            "shared": {
                "name": "Something for everyone",
                "thumbs": "",
                "original": ""
            }
        }
    }
}
```

The idea of a user and a users locations is embedded in the config file. This information is used for ALL server requests for data. This makes it impossible to access files outside the server data files.

for example to access any file via the server the following http request is required:

``` http
http://{serverpath}:{serverport}/files/user/{user}/loc/{loc}/path/{path}/name/{name}
```

Where:

| Item | description | example for 'user1' above |
| ----------- | ----------- | ----------- |
| {serverpath} | The server host address | ``` http://192.168.0.5 ``` |
| {serverport} | The server host port as defined with the serverport= server parameter | 8090 |
| {user} | The user name from the config file | 'user1' from above config data |
| {log} | The user location from the config file | 'dir1' or 'dir2' from above config data |
| {path} | see below | '.' or a further path |
| {name} | The file name | MyPic.jpg |

If the server is started as follows and starts on ip address and port '192.168.1.1:8090' with the above config data in 'config.json':

``` bash
thumbnails /home/user/data serverport=8090 serverconfig=config.json size=200 verbose
```

The follwoing request would download the full 'image1.jpg' from '/home/user/data/files1/images/set1'

``` link
http://192.168.1.1:8090/files/user/user1/loc/dir1/path/images%2Fset1/name/image1.jpg
```

Note: The '/' character requires 'escaping' (replacing with %2F). Where the hex ASCII value for '/' is 2F.

Spaces and other non http request compatible characters will also require 'escaping'.

This applies to both {path} and {name} parameters.

The following request would download the full image1.jpg from '/home/user/data/files2/docs'

``` link
http://192.168.1.1:8090/files/user/user1/loc/dir2/path/./name/image1.jpg
```

To download a thumbnail add the thumbnail query parameter to the request:

The follwoing request would download the thumbnail image1.jpg from '/home/user/data/files2/docs'. The size would be 200 as defined by the 'size=n' server parameter.

``` link
http://192.168.1.1:8090/files/user/user1/loc/dir2/path/./name/image1.jpg?thumbnail=true
```

The follwoing request would download the thumbnail image1.jpg from '/home/user/data/files2/docs'. The size would be 100 ignoring the 'size=n' server parameter.

``` link
http://192.168.1.1:8090/files/user/user1/loc/dir2/path/./name/image1.jpg?thumbnail=100
```

Will load the full image from source-path/a/b/image.jpg

All file types can be returned but only the following file types currently support thumbnail compression.

| File Extension | Content-Type |
| ----------- | ----------- |
| ".jpg" |   "image/jpeg" |
| ".jpeg" |  "image/jpeg" |
| ".png" |   "image/png" |


### Stopping the server

``` http
http://<ipaddress>:<port>/control/close
```

The server will close after 2 seconds.

## Thanks

ref: https://github.com/rwcarlsen/goexif (rwcarlsen) for the excelelent EXIF library.

ref: https://pkg.go.dev/github.com/liujiawm/graphics-go (liujiawm) for porting the graphics library from the original Google code.
