const require_index = require("../utils/index.js");
// lib/rules/static-class-names-order.ts
/**
* @fileoverview Alphabetizes static class names.
* @author Maciej Chmurski
*/
var static_class_names_order_default = {
	meta: {
		type: "suggestion",
		docs: {
			url: "https://eslint.vuejs.org/rules/static-class-names-order.html",
			description: "enforce static class names order",
			categories: void 0
		},
		fixable: "code",
		schema: [],
		messages: { shouldBeOrdered: "Classes should be ordered alphabetically." }
	},
	create: (context) => require_index.default.defineTemplateBodyVisitor(context, { "VAttribute[directive=false][key.name='class']"(node) {
		const value = node.value;
		if (!value) return;
		const classList = value.value;
		const classListWithWhitespace = classList.split(/(\s+)/);
		const divider = classListWithWhitespace.length > 1 ? classListWithWhitespace[1] : "";
		const classListSorted = classListWithWhitespace.filter((className) => className.trim() !== "").sort((a, b) => a.localeCompare(b)).join(divider);
		if (classList !== classListSorted) context.report({
			node,
			loc: node.loc,
			messageId: "shouldBeOrdered",
			fix: (fixer) => fixer.replaceText(value, `"${classListSorted}"`)
		});
	} })
};
exports.default = static_class_names_order_default;
