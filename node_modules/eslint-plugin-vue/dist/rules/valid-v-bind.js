const require_index = require("../utils/index.js");
// lib/rules/valid-v-bind.ts
/**
* @author Toru Nagashima
* @copyright 2017 Toru Nagashima. All rights reserved.
* See LICENSE file in root directory for full license.
*/
const VALID_MODIFIERS = /* @__PURE__ */ new Set([
	"prop",
	"camel",
	"sync",
	"attr"
]);
var valid_v_bind_default = {
	meta: {
		type: "problem",
		docs: {
			description: "enforce valid `v-bind` directives",
			categories: ["vue3-essential", "vue2-essential"],
			url: "https://eslint.vuejs.org/rules/valid-v-bind.html"
		},
		fixable: null,
		schema: [],
		messages: {
			unsupportedModifier: "'v-bind' directives don't support the modifier '{{name}}'.",
			expectedValue: "'v-bind' directives require an attribute value."
		}
	},
	create(context) {
		return require_index.default.defineTemplateBodyVisitor(context, { "VAttribute[directive=true][key.name.name='bind']"(node) {
			for (const modifier of node.key.modifiers) if (!VALID_MODIFIERS.has(modifier.name)) context.report({
				node: modifier,
				messageId: "unsupportedModifier",
				data: { name: modifier.name }
			});
			if (!node.value || require_index.default.isEmptyValueDirective(node, context)) context.report({
				node,
				messageId: "expectedValue"
			});
		} });
	}
};
exports.default = valid_v_bind_default;
