# Third-party data and protocol references

## GeoNames

`internal/geo/cities.csv.gz` is a reduced, gzip-compressed CSV derived from
[GeoNames](https://www.geonames.org/), licensed under
[Creative Commons Attribution 4.0](https://creativecommons.org/licenses/by/4.0/).
GeoNames provides the data without guarantees of accuracy or completeness.

The source is `cities15000.zip`: populated places with more than 15,000 people
or capitals. "Nearest city" means the nearest center **in this dataset**, which
can be a suburb or municipality and is not necessarily the containing city.
Country and administrative-region names come from `countryInfo.txt` and
`admin1CodesASCII.txt`. The reduction retains names/aliases, country, first-level
administrative region, coordinates, and IANA time zone.

Generated 2026-09-13, 34,136 places. Source SHA-256 values:

```text
admin1CodesASCII.txt 590651498043f674accda2b7f46d21286cda0e290b02f8561c5005eee9a5448c
countryInfo.txt      93bafc525813f22e4711ff9ed6d626343094ce48c26388dc7c49189b3d7d5512
cities15000.zip      15b9401f1e3216219bc58474a1d150c1c9e81dfbfb58e6188c96261f94a393db
```

Run `go generate ./internal/geo` to download current public data and regenerate
the reduced database; review the diff and update the date/checksums above.
The source feeds change daily, so those URLs do not reproduce older snapshots.
The committed compressed dataset pins the version used by a release.

## Time-zone boundaries and rules

[`github.com/ugjka/go-tz/v2`](https://github.com/ugjka/go-tz) embeds simplified
boundaries derived from [timezone-boundary-builder](https://github.com/evansiroky/timezone-boundary-builder/).
The lookup code is MIT licensed; boundary data is licensed under the
[Open Data Commons Open Database License 1.0](https://opendatacommons.org/licenses/odbl/1-0/).
The dependency contains the boundary database, source, and license. Source
versions are pinned in `go.mod` and `go.sum`. Simplified boundaries can be
inaccurate near borders; ambiguous/missing zones are reported unresolved.
Go's `time/tzdata` supplies IANA time-zone rules, including daylight saving time.

## Provider research

Google's undocumented request and array field layout were researched using
[`locationsharinglib`](https://github.com/costastf/locationsharinglib).
Apple Accessibility identifiers were researched using
[`FindMyFriends`](https://github.com/bytePatrol/FindMyFriends); the View → People
menu selection and bullet-separated freshness format were also checked against
[`findmy-cli`](https://github.com/omarshahine/findmy-cli).
These are unofficial integrations, independently implemented here.

The Google request's encoded `pb` parameter and field mapping originate from
locationsharinglib, whose license is reproduced below:

```text
Copyright 2017 Costas Tyfoxylos

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies
of the Software, and to permit persons to whom the Software is furnished to do
so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```
