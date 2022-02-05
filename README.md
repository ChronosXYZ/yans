# :newspaper: YANS

**YANS** (*Yet Another NNTP Server*) is a server implementation of NNTP protocol (according to [RFC 3977](https://datatracker.ietf.org/doc/html/rfc3977)) in Go.

## List of implemented commands and features

### Features

- :heavy_check_mark: Wildmat support
- :heavy_check_mark: Database (SQLite)
- :heavy_check_mark: Basic article posting
- :construction: Article retrieving
- :construction: Multipart article support
- :x: Transit mode
- :x: Authentication

#### Commands

- :heavy_check_mark: Session Administration Commands
  - :heavy_check_mark: `MODE READER`
  - :heavy_check_mark: `CAPABILITIES`
  - :heavy_check_mark: `QUIT`
- :construction: Article posting
  - :construction: `POST`
  - :x: `IHAVE`
- :heavy_check_mark: Article retrieving
  - :heavy_check_mark: `ARTICLE`
  - :heavy_check_mark: `HEAD`
  - :heavy_check_mark: `BODY`
  - :heavy_check_mark: `STAT`
- :construction: Articles overview
  - :heavy_check_mark: `OVER`
  - :x: `LIST OVERVIEW.FMT`
  - :x: `HDR`
  - :x: `LIST HEADERS`
- :heavy_check_mark: Group and Article Selection
  - :heavy_check_mark: `GROUP`
  - :heavy_check_mark: `LISTGROUP`
  - :heavy_check_mark: `LAST`
  - :heavy_check_mark: `NEXT`
- :construction: The LIST Commands
  - :heavy_check_mark: `LIST ACTIVE`
  - :heavy_check_mark: `LIST NEWSGROUPS`
  - :x: `LIST ACTIVE.TIMES`
  - :x: `LIST DISTRIB.PATS`
- :heavy_check_mark: Information Commands
  - :heavy_check_mark: `DATE`
  - :heavy_check_mark: `HELP`
  - :heavy_check_mark: `NEWGROUPS`
  - :heavy_check_mark: `NEWNEWS`

## License

This project is licensed under the GPLv3 license. For more information see [LICENSE](LICENSE) file.
