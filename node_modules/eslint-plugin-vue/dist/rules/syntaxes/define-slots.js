const require_index = require("../../utils/index.js");
// lib/rules/syntaxes/define-slots.ts
/**
* @author Yosuke Ota
* See LICENSE file in root directory for full license.
*/
var define_slots_default = {
	supported: ">=3.3.0",
	createScriptVisitor(context) {
		return require_index.default.defineScriptSetupVisitor(context, { onDefineSlotsEnter(node) {
			context.report({
				node,
				messageId: "forbiddenDefineSlots"
			});
		} });
	}
};
exports.default = define_slots_default;
