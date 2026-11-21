const require_index = require("../utils/index.js");
// lib/rules/require-toggle-inside-transition.ts
/**
* @author Yosuke Ota
* See LICENSE file in root directory for full license.
*/
function isValidBindAppear(vBindAppear) {
	return !vBindAppear.value?.expression || vBindAppear.value.expression.type !== "Literal" || vBindAppear.value.expression.value !== false;
}
function createDirectiveList(directives) {
	let str = "";
	for (const [index, directive] of directives.entries()) if (index === 0) str += `\`v-${directive}\``;
	else if (index < directives.length - 1) str += `, \`v-${directive}\``;
	else str += ` or \`v-${directive}\``;
	return str;
}
var require_toggle_inside_transition_default = {
	meta: {
		type: "problem",
		docs: {
			description: "require control the display of the content inside `<transition>`",
			categories: ["vue3-essential"],
			url: "https://eslint.vuejs.org/rules/require-toggle-inside-transition.html"
		},
		fixable: null,
		schema: [{
			type: "object",
			properties: { additionalDirectives: {
				type: "array",
				items: { type: "string" },
				uniqueItems: true
			} },
			additionalProperties: false
		}],
		messages: { expected: "The element inside `<transition>` is expected to have a {{allowedDirectives}} directive." }
	},
	create(context) {
		const allowedDirectives = [
			"if",
			"show",
			...context.options[0] && context.options[0].additionalDirectives || []
		];
		const allowedDirectivesString = createDirectiveList(allowedDirectives);
		/**
		* Check if the given element has display control.
		*/
		function verifyInsideElement(element) {
			if (require_index.default.isCustomComponent(element)) return;
			const parent = element.parent;
			const vBindAppear = require_index.default.getDirective(parent, "bind", "appear");
			if (require_index.default.hasAttribute(parent, "appear") || vBindAppear && isValidBindAppear(vBindAppear)) return;
			if (element.name !== "slot" && allowedDirectives.every((directive) => !require_index.default.hasDirective(element, directive)) && !require_index.default.hasDirective(element, "bind", "key")) context.report({
				node: element.startTag,
				loc: element.startTag.loc,
				messageId: "expected",
				data: { allowedDirectives: allowedDirectivesString }
			});
		}
		return require_index.default.defineTemplateBodyVisitor(context, { "VElement[name='transition'] > VElement"(node) {
			if (node.parent.children.find(require_index.default.isVElement) !== node) return;
			verifyInsideElement(node);
		} });
	}
};
exports.default = require_toggle_inside_transition_default;
