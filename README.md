# urlmd

An http API that ingests some url, and returns markdown from the html, caching the results


## Usage

```shell
git clone git@github.com:DeabLabs/urlmd.git
cd urlmd
docker compose up
curl http://localhost:8080/convert -X POST -d '{"url": "https://www.hackernews.com"}'
```
