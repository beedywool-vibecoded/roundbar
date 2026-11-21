// lib/rules/space-unary-ops.ts
/**
* @author Toru Nagashima
*/
var space_unary_ops_default = require("../utils/index.js").default.wrapStylisticOrCoreRule("space-unary-ops", {
	skipDynamicArguments: true,
	applyDocument: true
});
exports.default = space_unary_ops_default;
