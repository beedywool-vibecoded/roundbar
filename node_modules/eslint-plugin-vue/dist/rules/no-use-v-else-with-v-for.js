const require_index = require("../utils/index.js");
// lib/rules/no-use-v-else-with-v-for.ts
var no_use_v_else_with_v_for_default = {
	meta: {
		type: "suggestion",
		docs: {
			description: "disallow using `v-else-if`/`v-else` on the same element as `v-for`",
			categories: null,
			url: "https://eslint.vuejs.org/rules/no-use-v-else-with-v-for.html"
		},
		fixable: null,
		schema: [],
		messages: { unexpectedDirectiveWithVFor: "Unexpected `{{ directiveName }}` and `v-for` on the same element. Move `{{ directiveName }}` to a wrapper element instead." }
	},
	create(context) {
		return require_index.default.defineTemplateBodyVisitor(context, { "VAttribute[directive=true][key.name.name='for']"(node) {
			const element = node.parent.parent;
			if (require_index.default.hasDirective(element, "else-if")) context.report({
				node: element,
				messageId: "unexpectedDirectiveWithVFor",
				data: { directiveName: "v-else-if" }
			});
			else if (require_index.default.hasDirective(element, "else")) context.report({
				node: element,
				messageId: "unexpectedDirectiveWithVFor",
				data: { directiveName: "v-else" }
			});
		} });
	}
};
exports.default = no_use_v_else_with_v_for_default;
