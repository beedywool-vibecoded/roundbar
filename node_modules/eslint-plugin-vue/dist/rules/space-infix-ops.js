// lib/rules/space-infix-ops.ts
/**
* @author Toru Nagashima
*/
var space_infix_ops_default = require("../utils/index.js").default.wrapStylisticOrCoreRule("space-infix-ops", {
	skipDynamicArguments: true,
	applyDocument: true
});
exports.default = space_infix_ops_default;
