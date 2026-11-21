const require_index = require("../../utils/index.js");
// lib/rules/syntaxes/define-model.ts
/**
* @author Yosuke Ota
* See LICENSE file in root directory for full license.
*/
var define_model_default = {
	supported: ">=3.4.0",
	createScriptVisitor(context) {
		return require_index.default.defineScriptSetupVisitor(context, { onDefineModelEnter(node) {
			context.report({
				node,
				messageId: "forbiddenDefineModel"
			});
		} });
	}
};
exports.default = define_model_default;
