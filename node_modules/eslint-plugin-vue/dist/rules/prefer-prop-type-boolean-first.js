const require_index = require("../utils/index.js");
// lib/rules/prefer-prop-type-boolean-first.ts
function checkArrayExpression(node, context) {
	const booleanType = node.elements.find((element) => element && element.type === "Identifier" && element.name === "Boolean");
	if (!booleanType) return;
	const booleanTypeIndex = node.elements.indexOf(booleanType);
	if (booleanTypeIndex > 0) context.report({
		node: booleanType,
		messageId: "shouldBeFirst",
		suggest: [{
			messageId: "moveToFirst",
			fix: (fixer) => {
				const sourceCode = context.sourceCode;
				const elements = [...node.elements];
				elements.splice(booleanTypeIndex, 1);
				const code = elements.filter(require_index.default.isDef).map((element) => sourceCode.getText(element));
				code.unshift("Boolean");
				return fixer.replaceText(node, `[${code.join(", ")}]`);
			}
		}]
	});
}
var prefer_prop_type_boolean_first_default = {
	meta: {
		type: "problem",
		docs: {
			description: "enforce `Boolean` comes first in component prop types",
			categories: void 0,
			url: "https://eslint.vuejs.org/rules/prefer-prop-type-boolean-first.html"
		},
		fixable: null,
		hasSuggestions: true,
		schema: [],
		messages: {
			shouldBeFirst: "Type `Boolean` should be at first in prop types.",
			moveToFirst: "Move `Boolean` to be first in prop types."
		}
	},
	create(context) {
		function checkProperty(prop) {
			if (prop.type !== "object") return;
			const { value } = prop;
			if (!value) return;
			if (value.type === "ArrayExpression") checkArrayExpression(value, context);
			else if (value.type === "ObjectExpression") {
				const typeProperty = require_index.default.findProperty(value, "type");
				if (!typeProperty || typeProperty.value.type !== "ArrayExpression") return;
				checkArrayExpression(typeProperty.value, context);
			}
		}
		return require_index.default.compositingVisitors(require_index.default.defineScriptSetupVisitor(context, { onDefinePropsEnter(_, props) {
			for (const prop of props) checkProperty(prop);
		} }), require_index.default.executeOnVue(context, (obj) => {
			const props = require_index.default.getComponentPropsFromOptions(obj);
			for (const prop of props) checkProperty(prop);
		}));
	}
};
exports.default = prefer_prop_type_boolean_first_default;
