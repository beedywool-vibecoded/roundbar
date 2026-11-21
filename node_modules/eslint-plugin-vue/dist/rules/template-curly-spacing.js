// lib/rules/template-curly-spacing.ts
/**
* @author Yosuke Ota
*/
var template_curly_spacing_default = require("../utils/index.js").default.wrapStylisticOrCoreRule("template-curly-spacing", {
	skipDynamicArguments: true,
	applyDocument: true
});
exports.default = template_curly_spacing_default;
