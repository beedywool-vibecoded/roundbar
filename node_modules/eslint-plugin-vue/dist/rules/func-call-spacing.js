// lib/rules/func-call-spacing.ts
/**
* @author Yosuke Ota
*/
var func_call_spacing_default = require("../utils/index.js").default.wrapStylisticOrCoreRule({
	core: "func-call-spacing",
	stylistic: "function-call-spacing",
	vue: "func-call-spacing"
}, {
	skipDynamicArguments: true,
	applyDocument: true
});
exports.default = func_call_spacing_default;
