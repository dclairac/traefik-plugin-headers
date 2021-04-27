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
            responseHeaders:
              Expires:
                value: '@DT_ADD#86400@'
                action: 'set'
          - rule:
            name: 'If no rules match'
            regexp: 'NO_MATCH'
            requestHeaders:
              header-to-delete:
                action: 'unset'
                    
```

## How to use

### Base structure
The plugin expects a list of `rules` with for each of them:
 * **name:** The name of the rule
 * **regexp:** Regular expression to determine if the rule should apply to the query 
 * **requestHeaders:** List of request headers to modify if the regexp match
 * **responseHeaders:** List of response headers to modify if the regexp match


The rules are tested and applied **in the order of declaration in the configuration**. If several rules modify the same header, it will take the value of the last rule declared in the configuration.

It is possible to specify the value "`NO_MATCH`" (case sensitive) in the regexp attribute. The rule will only be applied **if no other rule has been applied before the `NO_MATCH` is encountered**, then the rule evaluation will continue.

If several `NO_MATCH` rules are defined, **only the first one will have a chance to be executed**, the others will be useless (either a rule will have already been applied, or the 1st `NO_MATCH` will have been applied, which will prevent the execution of a 2nd `NO_MATCH` in any case)

`requestHeaders` and `responseHeaders` will receive the map of `headers` to modify with the expected `actions` 

### Headers declarations

For each header, the following attributes can be defined:
* **value:** Value to write in the header
* **replace:** `Regexp` indicating the value to modify in the header (only for `edit`action)
*	**action:**  Action to perform (`set`, `unset`, `edit`, `append`)

## Headers `action`

### Action `set` - Add or Replace header 

The `set` action allows to add the `header` by giving it the indicated `value`. If the `header` already exists, its value will be replaced by the new one. (`header` name **is case insensitive**). 

```yaml
#Example
- rule:
  name: 'Set X-foo=bar Header to js requests'
  regexp: '\.js$'
  requestHeaders: 
    X-foo: 
      value: 'bar'
      action: 'set'
```
The `X-foo=bar` header will be added to requests ending in **.js**. If the `X-foo` header already exists with another value, it will be overwritten by the new value: `bar` 

### Action `unset` - delete selected header 

The `unset` action will delete the `header`. (`header` name **is case insensitive**). 

```yaml
#Example
- rule:
  name: 'Unset X-foo Header to js requests'
  regexp: '\.js$'
  requestHeaders: 
    X-foo: 
      action: 'unset'
```
The `X-foo` header will be deleted to requests ending in **.js**. If the `X-foo` header did not exist, the rule will have no effect and will be ignored.

### Action `edit` - Modify content of selected header 

the `edit` action will modify the content of `header` based on regexp evaluation. (`header` **is case insensitive**.).

Basically, the following actions will be executed:
* If the `header` does not already exist, it will be added with `value` content
* If the `header` already exists:
  * If the regexp `replace` match, it will be replaced by `value` without any other change 
  * If the regexp does not match, `value` content will be added to the existing header
```yaml
#Example
- rule:
  name: 'edit Cache-Control Header to js requests'
  regexp: '\.js$'
  requestHeaders:
    Cache-Control: 
      value: 'max-age=1000'
      replace: 'max-age=[0-9]+'
      action: 'edit'
```
If `Cache-Control` did not exist, it **will be created** with the value `max-age=1000`. If `Cache-Control` existed with the value `no-cache`, the value `max-age=1000` **will be added**. If `Cache-Control` existed with the value `no-cache, max-age=0, must-revalidate`, the value **will be changed** to `no-cache, max-age=1000, must-revalidate`.  

### Action `append` - add value to selected header 

the `append` action will add the `value` attribute to the `header`. (`header` **is case insensitive**). If the `header` was not existing, it will be added.
```yaml
#Example
- rule:
  name: 'Append X-foo=bar Header to js requests'
  regexp: '\.js$'
  requestHeaders:
    X-foo: 
      value: 'bar'
      action: 'append'
```
The `X-foo=bar` header will be added to requests ending in **.js**. If the `X-foo` header already exists with another value, new value will be added without any change on existing one.

## Date manipulation
Whatever the action (except `unset`), it is possible to add a replacement sequence in the `value` attribute. This sequence allows to calculate a date in the future. The sequence will be replaced by the date in http format. This is particularly interesting if you want to reproduce the behavior of the mod_expires of the Apache httpd server
```yaml
#Example
- rule:
  name: 'Set Expires in 1 day on js query'
  regexp: '\.js$'
  requestHeaders:
    Expires: 
      value: '@DT_ADD#86400@'
      action: 'set'
```
Sequence is construct with **@DT_ADD#xxx@** with `xxx` the number of seconds to add to the time it will be when the request is processed.

If the above request is executed on April 10, 2021 at 10:42:00, the folowwing header will be set to the response **(as 86400 seconds equals to 1 day)**
`Expires: Sun, 11 Apr 2021 10:42:00 GMT`.

If sequence is surround with other text values, only sequence will be replaced by the plugin. if `We are @DT_ADD#86400@ - have a nice day!` is used as `value`, result header will be: `We are Sun, 11 Apr 2021 10:42:00 GMT - have a nice day!`


## WARNING

### Performances
For each query, all the rules will be tested. The addition of a very large number of rules can therefore **have a negative impact on the response time of requests**

### Execution order
The different rules will be evaluated **in the order in which they are defined**, as will the header changes. Several rules **can be applied successively to the same request**.
`NO_MATCH` rule will only be applied if no rule has been used before on request.

### Multiple header
The **HTTP** standard indicates that it is possible to assign several values to the same header. To do this, you must either define the header and place all the values by separating them with a comma (`MyHeader=val1, Val2 ...`), or by declaring the header several times (`MyHeader=val1` and `MyHeader=val2`). The two notations are equivalent and the **plugin uses the second one**.

# Authors
* **dclairac** ([linkedin](https://www.linkedin.com/in/dclairac/))