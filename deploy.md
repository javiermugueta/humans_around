#
```
fn create context humans --provider oracle
fn use context humans
fn update context  oracle.compartment-id ocid1.compart......m5wa
fn update context  api-url https://functions.eu-frankfurt-1.oraclecloud.com
fn update context  registry fra.ocir.io/<namespace>/humans
docker login -u '<tenancyname>/oracleidentitycloudservice/<user>' fra.ocir.io
<the password here is an auth token created by the user>
fn deploy --app humans_around  --verbose --no-bump
```