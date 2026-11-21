// lib/rules/comma-spacing.ts
/**
* @author Yosuke Ota
*/
var comma_spacing_default = require("../utils/index.js").default.wrapStylisticOrCoreRule("comma-spacing", {
	skipDynamicArguments: true,
	skipDynamicArgumentsReport: true,
	applyDocument: true
});
exports.default = comma_spacing_default;
