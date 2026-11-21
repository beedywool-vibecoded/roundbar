const require_index = require("../utils/index.js");
// lib/rules/no-multiple-objects-in-class.ts
/**
* @author tyankatsu <https://github.com/tyankatsu0105>
* See LICENSE file in root directory for full license.
*/
/**
* count ObjectExpression element
*/
function countObjectExpression(node) {
	return node.value.expression.elements.filter((element) => element && element.type === "ObjectExpression").length;
}
var no_multiple_objects_in_class_default = {
	meta: {
		type: "suggestion",
		docs: {
			description: "disallow passing multiple objects in an array to class",
			categories: void 0,
			url: "https://eslint.vuejs.org/rules/no-multiple-objects-in-class.html"
		},
		fixable: null,
		schema: [],
		messages: { unexpected: "Unexpected multiple objects. Merge objects." }
	},
	create(context) {
		return require_index.default.defineTemplateBodyVisitor(context, { "VAttribute[directive=true][key.argument.name=\"class\"][key.name.name=\"bind\"][value.expression.type=\"ArrayExpression\"]"(node) {
			if (countObjectExpression(node) > 1) context.report({
				node,
				loc: node.loc,
				messageId: "unexpected"
			});
		} });
	}
};
exports.default = no_multiple_objects_in_class_default;
