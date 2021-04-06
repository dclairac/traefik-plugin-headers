# Manage request and response headers

This plugin allows you to define rules to **add**, **remove** or **modify** request headers **based on the URI**. It also allows to reproduce the behavior of the apache *mod_expires* for the definition of a validity period of the resource.

## Configuration
### Static configuration (`traefik.yml`)

```yml
pilot:
  token: [REDACTED]

experimental:
  plugins:
    traefik-plugin-header:
      moduleName: github.com/dclairac/traefik-plugin-headers
      version: v1.0.0
```

### Dynamic configuration
```yaml
http:
  routers:
    my-router:
      rule: Host(`www.my-site.com`)
      service: my-site
      entryPoints:
        - http
      middlewares:
        - traefik-plugin-headers

  services:
   my-site:
      loadBalancer:
        servers:
          - url: http://127.0.0.1:3163
  
  middlewares:
    traefik-plugin-headers:
      plugin:
        defaultHeaders:
          - headerChange:
            header: 'header-to-delete'
            req: true
            action: 'unset'
        rules:
          - rule:
            name: 'Only for png or js'
            regexp: '(png|js)$'
            headerChanges:
              - headerChange:
                name: 'Add response header Expires at now + 1 Day'
                header: 'Expires'
                req: false
                value: '@DT_ADD#86400@'
                action: 'set'            
```

## How to use

### Base structure
The plugin allow the creation of 2  `headerChange` lists:
 * **rules:** Rules to determine (`regexp`) the requests where the `headerChange` will be applied
 * **defaultHeaders:** List of `headerChange` to apply only on requests not affected by any rules

The following example will add an `example` header to the response of png or js files, and remove the `example` header from the responses of all other requests

```yaml
traefik-plugin-headers:
  plugin:
    defaultHeaders:
      - headerChange:
        header: 'example'
        type: 'unset'
    rules:
      - rule:
        regexp: '(png|js)$'
        headerChanges:
          - headerChange:
            header: 'example'
            value: 'PNG OR JS'
            type: 'set'            
```

**WARNING:** Default `headerChange` are performed on **ALL** requests (no regexp) but only if they are not concerned by **ANY** other rule.

### HeaderChange Structure

`HeaderChange` can receive the following attributes: 
```yaml
headerChange:
  name:    'Name'     # [optional] Name of the headerChange
  header:  'Expires'  # Name of the header to modify
  req:     false      # true = modify request header, false (default) modify response header
  value:   'my value' # Content of the header (depends on action)           
  replace: 'a{2}'     # Only for action edit - see edit documentation
  sep: ', '           # Separator to use whe you append value to existing header
  action: 'set'       # set/unset/append/edit => see documentation
```

## List of possible `action`

### Action `set` - Add or replace header 

the `set` action allows to add the `header` by giving it the indicated `value`. If the `header` already exists, its value will be replaced by the new one. `header` **is case insensitive**. 

**Required** attributes for `set`:
* `header`  : header to add/modify
* `value`   : value that will be set to the header
* `action`  : must be 'set' to perform `set`action 

**Optional** attributes for `set`:
* `name`    : name of the change. Only used for logs
* `req`     : boolean => **true** = request / **false** *(default)* = response

### Action `unset` - delete selected header 

the `unset` action will delete the request/response header that correspond to `header`attribute. `header` **is case insensitive**. 

**Required** attributes for `unset`:
* `header`  : header to delete
* `action`  : must be 'unset' to perform `unset`action 

**Optional** attributes for `unset`:
* `name`    : name of the change. Only used for logs
* `req`     : boolean => **true** = request / **false** *(default)* = response

### Action `edit` - Modify content of selected header 

the `edit` action will modify the content of header value based on regexp evaluation. 
Basically, the following actions will be executed:
* If the `header` does not already exist, it will be added with its `value`
* If the `header` already exists:
  * If the regexp `replace` match, it is replaced by `value`without any other change 
  * If the regexp does not match, `value` is added at the end of with the `sep` separator
`header` **is case insensitive**. 

**Required** attributes for `edit`:
* `header`  : header to be modified
* `value`   : value to be added to the header
* `replace` : Go regexp to locate the area to be replaced
* `sep`     : separator to use if the value must be added at the end of the header
* `action`  : must be 'edit' to perform `edit`action 

**Optional** attributes for `edit`:
* `name`    : name of the change. Only used for logs
* `req`     : boolean => **true** = request / **false** *(default)* = response

**example:**
```yaml
headerChange:
  header:  'Cache-Control'
  value:   'max-age=1000'
  replace: 'max-age=[0-9]+'
  sep: ', '           # Separator to use whe you append value to existing header
  action: 'edit'       # set/unset/append/edit => see documentation
```
*This will give the following results in the headers of the response:*
* If the *Cache-Control* header does not exist:
  * Response header:  `Cache-Control: max-age=1000`
* If the *Cache-Control* header exist with value: ` no-store, max-age=0, no-cache`
  * Response header:  `Cache-Control: no-store, max-age=1000, no-cache`
* If the *Cache-Control* header exist with value: ` no-store, no-cache`
  * Response header:  `Cache-Control: no-store, no-cache, max-age=1000`

### Action `append` - add value to selected header 

the `append` action will add the `value` attribute to the `header`. `header` **is case insensitive**. The standard allows the addition of a value either by concatenating it with the existing `value`, or by adding a second `header` with the same name. The `append` action allows both, depending on the presence or absence of the `sep` separator. If it is present, the value will be concatenated, otherwise a `header` with the same name will be added. 
If the `header` was not existing, it will be added.

**Required** attributes for `append`:
- `header`  : header to modify
* `value`   : value to be added to the header
* `sep`     : separator to use if you prefer to concatenate the value to existing one
- `action`  : must be 'append' to perform `append`action 

**Optional** attributes for `append`:
- `name`    : name of the change. Only used for logs
- `req`     : boolean => **true** = request / **false** *(default)* = response


## Date manipulation
Whatever the action (except `unset`), it is possible to add a replacement sequence in the `value` attribute. This sequence allows to calculate a date in the future. The sequence will be replaced by the date in http format. This is particularly interesting if you want to reproduce the behavior of the mod_expires of the Apache httpd server

**example:**
```yaml
headerChange:
  header:  'Expires'
  value:   '@DT_ADD#86400@'
  action:  'set'
```
Sequence is construct with **@DT_ADD#xxx@** with `xxx` the number of seconds to add to the time it will be when the request is processed.

If the above request is executed on April 10, 2021 at 10:42:00, the folowwing header will be added to the response **(as 86400 seconds equals to 1 day)**
`Expires: Sun, 11 Apr 2021 10:42:00 GMT`.

If sequence is surround with other text values, only sequence will be replaced by the plugin. if `We are @DT_ADD#86400@ - have a nice day!` is used as `value`, result header will be: `We are Sun, 11 Apr 2021 10:42:00 GMT - have a nice day!`

## WARNING

### Performances
For each query, all the rules will be tested. The addition of a very large number of rules can therefore **have a negative impact on the response time of requests**

### Execution order
The different rules will be evaluated **in the order in which they are defined**, as will the header changes. Several rules **can be applied successively to the same request**.
`defaultHeaders` modifications are only applied if no rule has been used on request.

```yaml
#Example
- Rule:
  Name: 'Header addition'
  Header: 'X-Custom-2'
  Value: 'True'
  Type: 'Set'
- Rule:
  Name: 'Header deletion'
  Header: 'X-Custom-2'
  Type: 'Del'
- Rule:
  Name: 'Header join'
  Header: 'X-Custom-2'
  Value: 'False'
  Type: 'Set'
```
Will firstly set the header `X-Custom-2` to 'True', then delete it and lastly set it again but with `False`

# Authors
* **dclairac** ([linkedin](https://www.linkedin.com/in/dclairac/))