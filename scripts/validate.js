const fs = require("fs");
const { buildClientSchema, parse, validate } = require("graphql");
const { getVariableValues } = require("graphql/execution/values");

const schema = buildClientSchema(JSON.parse(fs.readFileSync(process.argv[3])).data);
// Shopify's schema trips graphql-js's own deprecation rules; skip that check.
schema.__validationErrors = [];

const caps = JSON.parse(fs.readFileSync(process.argv[2]));
let bad = 0;
for (const c of caps) {
  let errs = [];
  let doc;
  try {
    doc = parse(c.query);
    errs = [...validate(schema, doc)];
  } catch (e) {
    errs = [e];
  }
  if (doc && !errs.length) {
    const op = doc.definitions.find((d) => d.kind === "OperationDefinition");
    const r = getVariableValues(schema, op.variableDefinitions || [], c.variables || {});
    if (r.errors) errs.push(...r.errors);
  }
  if (errs.length) {
    bad++;
    console.log("FAIL", c.name);
    errs.forEach((e) => console.log("   ", e.message));
  }
}
console.log(`${caps.length} requests, ${bad} failed`);
process.exit(bad ? 1 : 0);
