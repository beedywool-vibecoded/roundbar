// lib/rules/comma-style.ts
/**
* @author Yosuke Ota
*/
var comma_style_default = require("../utils/index.js").default.wrapStylisticOrCoreRule("comma-style", { create(_context, { baseHandlers }) {
	return { VSlotScopeExpression(node) {
		if (baseHandlers.FunctionExpression) baseHandlers.FunctionExpression(node);
	} };
} });
exports.default = comma_style_default;
