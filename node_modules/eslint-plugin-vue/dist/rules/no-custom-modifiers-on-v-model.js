const require_index = require("../utils/index.js");
// lib/rules/no-custom-modifiers-on-v-model.ts
/**
* @author Przemyslaw Falowski (@przemkow)
* @fileoverview This rule checks whether v-model used on the component do not have custom modifiers
*/
const VALID_MODIFIERS = /* @__PURE__ */ new Set([
	"lazy",
	"number",
	"trim"
]);
var no_custom_modifiers_on_v_model_default = {
	meta: {
		type: "problem",
		docs: {
			description: "disallow custom modifiers on v-model used on the component",
			categories: ["vue2-essential"],
			url: "https://eslint.vuejs.org/rules/no-custom-modifiers-on-v-model.html"
		},
		fixable: null,
		schema: [],
		messages: { notSupportedModifier: "'v-model' directives don't support the modifier '{{name}}'." }
	},
	create(context) {
		return require_index.default.defineTemplateBodyVisitor(context, { "VAttribute[directive=true][key.name.name='model']"(node) {
			const element = node.parent.parent;
			if (require_index.default.isCustomComponent(element)) {
				for (const modifier of node.key.modifiers) if (!VALID_MODIFIERS.has(modifier.name)) context.report({
					node,
					loc: node.loc,
					messageId: "notSupportedModifier",
					data: { name: modifier.name }
				});
			}
		} });
	}
};
exports.default = no_custom_modifiers_on_v_model_default;
