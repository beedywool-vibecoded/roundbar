const require_index = require("../../utils/index.js");
// lib/rules/syntaxes/v-bind-same-name-shorthand.ts
/**
* @author Yosuke Ota
* See LICENSE file in root directory for full license.
*/
var v_bind_same_name_shorthand_default = {
	supported: ">=3.4.0",
	createTemplateBodyVisitor(context) {
		/**
		* Verify the directive node
		*/
		function checkDirective(node) {
			if (require_index.default.isVBindSameNameShorthand(node)) context.report({
				node,
				messageId: "forbiddenVBindSameNameShorthand",
				fix: (fixer) => fixer.insertTextAfter(node, `="${node.value.expression.name}"`)
			});
		}
		return { "VAttribute[directive=true][key.name.name='bind']": checkDirective };
	}
};
exports.default = v_bind_same_name_shorthand_default;
