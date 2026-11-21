const require_index = require("../utils/index.js");
// lib/rules/require-prop-type-constructor.ts
const forbiddenTypes = /* @__PURE__ */ new Set([
	"Literal",
	"TemplateLiteral",
	"BinaryExpression",
	"UpdateExpression"
]);
function isForbiddenType(node) {
	return forbiddenTypes.has(node.type) && !(node.type === "Literal" && node.value == null && !node.bigint);
}
var require_prop_type_constructor_default = {
	meta: {
		type: "suggestion",
		docs: {
			description: "require prop type to be a constructor",
			categories: ["vue3-essential", "vue2-essential"],
			url: "https://eslint.vuejs.org/rules/require-prop-type-constructor.html"
		},
		fixable: "code",
		schema: [],
		messages: { propTypeConstructor: "The \"{{name}}\" property should be a constructor." }
	},
	create(context) {
		function checkPropertyNode(propName, node) {
			const nodes = node.type === "ArrayExpression" ? node.elements.filter(require_index.default.isDef) : [node];
			for (const prop of nodes) {
				if (!isForbiddenType(prop)) continue;
				context.report({
					node: prop,
					messageId: "propTypeConstructor",
					data: { name: propName },
					fix: (fixer) => {
						if (prop.type === "Literal" || prop.type === "TemplateLiteral") {
							const newText = require_index.default.getStringLiteralValue(prop, true);
							if (newText) return fixer.replaceText(prop, newText);
						}
						return null;
					}
				});
			}
		}
		function verifyProps(props) {
			for (const prop of props) {
				if (prop.type !== "object" || prop.propName == null) continue;
				if (isForbiddenType(prop.value) || prop.value.type === "ArrayExpression") checkPropertyNode(prop.propName, prop.value);
				else if (prop.value.type === "ObjectExpression") {
					const typeProperty = require_index.default.findProperty(prop.value, "type");
					if (!typeProperty) continue;
					checkPropertyNode(prop.propName, typeProperty.value);
				}
			}
		}
		return require_index.default.compositingVisitors(require_index.default.defineScriptSetupVisitor(context, { onDefinePropsEnter(_node, props) {
			verifyProps(props);
		} }), require_index.default.executeOnVueComponent(context, (obj) => {
			verifyProps(require_index.default.getComponentPropsFromOptions(obj));
		}));
	}
};
exports.default = require_prop_type_constructor_default;
