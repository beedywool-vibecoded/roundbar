const require_index = require("../utils/index.js");
// lib/rules/valid-v-is.ts
/**
* @fileoverview enforce valid `v-is` directives
* @author Yosuke Ota
*/
/**
* Check whether the given node is valid or not.
*/
function isValidElement(node) {
	return !require_index.default.isHtmlElementNode(node) || require_index.default.isHtmlWellKnownElementName(node.rawName);
}
var valid_v_is_default = {
	meta: {
		type: "problem",
		docs: {
			description: "enforce valid `v-is` directives",
			categories: ["vue3-essential"],
			url: "https://eslint.vuejs.org/rules/valid-v-is.html"
		},
		fixable: null,
		schema: [],
		messages: {
			unexpectedArgument: "'v-is' directives require no argument.",
			unexpectedModifier: "'v-is' directives require no modifier.",
			expectedValue: "'v-is' directives require that attribute value.",
			ownerMustBeHTMLElement: "'v-is' directive must be owned by a native HTML element, but '{{name}}' is not."
		}
	},
	create(context) {
		return require_index.default.defineTemplateBodyVisitor(context, { "VAttribute[directive=true][key.name.name='is']"(node) {
			if (node.key.argument) context.report({
				node: node.key.argument,
				messageId: "unexpectedArgument"
			});
			const lastModifier = node.key.modifiers.at(-1);
			if (lastModifier) context.report({
				node,
				loc: {
					start: node.key.modifiers[0].loc.start,
					end: lastModifier.loc.end
				},
				messageId: "unexpectedModifier"
			});
			if (!node.value || require_index.default.isEmptyValueDirective(node, context)) context.report({
				node,
				messageId: "expectedValue"
			});
			const element = node.parent.parent;
			if (isValidElement(element)) return;
			const name = element.name;
			context.report({
				node,
				messageId: "ownerMustBeHTMLElement",
				data: { name }
			});
		} });
	}
};
exports.default = valid_v_is_default;
