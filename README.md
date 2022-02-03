# :newspaper: YANS

**YANS** (*Yet Another NNTP Server*) is a server implementation of NNTP protocol (according to [RFC 3977](https://datatracker.ietf.org/doc/html/rfc3977)) in Go.

## List of implemented commands and features

### Features

- :heavy_check_mark: Wildmat support
- :heavy_check_mark: Database (SQLite)
- :heavy_check_mark: Basic article posting
- :construction: Multipart articles support
- :x: Transit mode
- :x: Authentication

#### Commands
- :heavy_check_mark: `CAPABILITIES`
- :heavy_check_mark: `DATE`
- :heavy_check_mark: `LIST ACTIVE`
- :heavy_check_mark: `LIST NEWSGROUPS`
- :heavy_check_mark: `MODE READER`
- :heavy_check_mark: `QUIT`
- :x: `ARTICLE`
- :x: `BODY`
- :heavy_check_mark: `GROUP`
- :x: `HDR`
- :x: `HEAD`
- :x: `HELP`
- :x: `IHAVE`
- :x: `LAST`
- :heavy_check_mark: `LISTGROUP`
- :heavy_check_mark: `NEWGROUPS`
- :x: `NEWNEWS`
- :x: `NEXT`
- :x: `OVER`
- :construction: `POST`
- :x: `STAT`

## License

This project is licensed under the GPLv3 license. For more information see [LICENSE](LICENSE) file.
