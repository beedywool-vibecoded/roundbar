// lib/rules/space-in-parens.ts
/**
* @author Yosuke Ota
*/
var space_in_parens_default = require("../utils/index.js").default.wrapStylisticOrCoreRule("space-in-parens", {
	skipDynamicArguments: true,
	skipDynamicArgumentsReport: true,
	applyDocument: true
});
exports.default = space_in_parens_default;
