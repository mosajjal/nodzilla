# NodZilla

an API for newly observed domains

## Configuration

refer to `config.defaults.yaml` in the root of the repo to see all the options

## Usage

```bash
nodzilla provides a ReST API for newly observed domains

Usage:
  nodzilla [flags]

Flags:
  -c, --config string     path to YAML configuration file (default "$HOME/.nodzilla.yaml")
  -d, --defaultconfig     write default config to $HOME/.nodzilla.yaml
  -h, --help              help for nodzilla
  -l, --loglevel string   log level (debug, info, warn, error, fatal, panic) (default "info")
  -V, --version           print version and exit
```

## API Endpoints

- Query Single Domain
```bash
curl -XGET -u USERNAME:PASSWORD http://127.0.0.1:3000/query/www.newdomain.com
```

- Query Multiple Domains

```bash
curl -XGET -u USERNAME:PASSWORD http://127.0.0.1:3000/query_many -H 'Content-Type: application/json' -d '["domain1.com","domain2.com"]'
```

- Add a Domain to the Database. Note that the Datetime is always RFC3339

```bash
curl -XPOST http://127.0.0.1:3000/admin/add_domain -H 'Content-Type: application/json' -d '{"domain":"domain.com","registration_date":"2020-01-01T12:00:00+12:00"}'
```

- Add Multiple Domains to the Database

```bash
curl -XPOST -u "USERNAME:PASSWORD" http://127.0.0.1:3000/admin/add_domains -H 'Content-Type: application/json' -d '[{"domain":"domain.com","registration_date":"2020-01-01T12:00:00+12:00"},{"domain":"domain2.com","registration_date":"2020-02-01T12:00:00+12:00"}]'
```

- Delete a Domain from the Database

```bash
curl -XDELETE -u "USERNAME:PASSWORD" http://127.0.0.1:3000/admin/delete_domain/google.com -H "Content-Type: application/json"
```

- Delete Multiple Domains from the Database

```bash
curl -XDELETE -u "USERNAME:PASSWORD" http://127.0.0.1:3000/admin/delete_domains -H "Content-Type: application/json" -d '["bing.com","google.com"]'
```

My recommendation is, batch the inserts and deletes into 100-1000 in each call rather than doing it all in one call. 


## FAQ

Q: Where do I find newly observed domains?

A: https://www.whoisdownload.com/ has a free feed. You can also use a paid service like https://domains-monitor.com/

Q: What is the use case of this API

A: DGA, Phishing and some malware C2s use new domains to communicate. Having a fast API to query domains in real time and alert on endpoints talking to a NOD helps the defenders detect and prevent credential harvesting and malicious activities

Q: How do I keep my database up-to-date

A: nodzilla will need a companion script to periodically download the new domains of the day, create POST requests in 100-1000 batches and POST it to nodzilla. Here's a simple script to do this in bash

```python

import requests
import datetime
import os

APIKEY= os.environ.get('APIKEY')

url = f"https://domains-monitor.com/api/v1/{APIKEY}/get/dailyupdate/list/text/"
BATCH_SIZE = 100
# get today's date in RFC3339
today = datetime.datetime.utcnow().strftime("%Y-%m-%dT%H:%M:%S.%fZ")
def get_stream(url):
    s = requests.Session()
    send_to_nodzilla = []
    cnt = 0
    with s.get(url, headers=None, stream=True) as resp:
        for line in resp.iter_lines():
            if line:
                cnt += 1
                line = line.decode('utf-8')
                tmp = {"domain": line, "registration_date": today}
                send_to_nodzilla.append(tmp)
                if cnt == BATCH_SIZE:
                    yield send_to_nodzilla
                    send_to_nodzilla = []
                    cnt = 0


for batch in get_stream(url):
    res = requests.post('http://127.0.0.1:3000/admin/add_domains', json=batch, auth=('username1', 'password1'))
    print(res.status_code)

```

Q: How do I delete older domains from the database

A: currently, there is no way to perform delete based on the date they were Added, or their registration date. Raise an issue if this is a requirement so the feature gets developed

Q: How to have stronger authentication

A: disable the builtin basic auth, and put the server behind Cloudflare tunnels, or any other service that provides authentication. You can also use `oauth2proxy` or Okta

Q: How do I integrate this tool with my SIEM or network security monitoring solution

A: you can directly access the database using any `pebble` CLI, or use the ReST API to build your automation and SOAR

Q: I'm getting 429 when I try to query multiple times a second

A: there's a builtin rate limiter (request per second). You can disable that by setting rps to 0 in the config.yaml file
