const require_index = require("../utils/index.js");
// lib/rules/require-default-prop.ts
const NATIVE_TYPES = /* @__PURE__ */ new Set([
	"String",
	"Number",
	"Boolean",
	"Function",
	"Object",
	"Array",
	"Symbol"
]);
/**
* Detects whether given value node is a Boolean type
*/
function isValueNodeOfBooleanType(value) {
	if (value.type === "Identifier" && value.name === "Boolean") return true;
	if (value.type === "ArrayExpression") {
		const elements = value.elements.filter(require_index.default.isDef);
		return elements.length === 1 && elements[0].type === "Identifier" && elements[0].name === "Boolean";
	}
	return false;
}
var require_default_prop_default = {
	meta: {
		type: "suggestion",
		docs: {
			description: "require default value for props",
			categories: ["vue3-strongly-recommended", "vue2-strongly-recommended"],
			url: "https://eslint.vuejs.org/rules/require-default-prop.html"
		},
		fixable: null,
		schema: [],
		messages: { missingDefault: `Prop '{{propName}}' requires default value to be set.` }
	},
	create(context) {
		/**
		* Checks if the passed prop is required
		*/
		function propIsRequired(propValue) {
			return propValue.properties.some((p) => p.type === "Property" && require_index.default.getStaticPropertyName(p) === "required" && p.value.type === "Literal" && p.value.value === true);
		}
		/**
		* Checks if the passed prop has a default value
		*/
		function propHasDefault(propValue) {
			return propValue.properties.some((p) => p.type === "Property" && require_index.default.getStaticPropertyName(p) === "default");
		}
		/**
		* Checks whether the given props that don't have a default value
		*/
		function isWithoutDefaultValue(prop) {
			switch (prop.value.type) {
				case "ObjectExpression": return !propIsRequired(prop.value) && !propHasDefault(prop.value);
				case "Identifier": return NATIVE_TYPES.has(prop.value.name);
				case "CallExpression":
				case "MemberExpression": return false;
				default: return true;
			}
		}
		/**
		* Detects whether given prop node is a Boolean
		*/
		function isBooleanProp(prop) {
			const value = require_index.default.skipTSAsExpression(prop.value);
			return isValueNodeOfBooleanType(value) || value.type === "ObjectExpression" && value.properties.some((p) => p.type === "Property" && p.key.type === "Identifier" && p.key.name === "type" && isValueNodeOfBooleanType(p.value));
		}
		function processProps(props, shouldIgnore) {
			for (const prop of props) if (prop.type === "object") {
				if (prop.node.shorthand || !isWithoutDefaultValue(prop) || isBooleanProp(prop) || shouldIgnore?.(prop)) continue;
				const propName = prop.propName ?? `[${context.sourceCode.getText(prop.node.key)}]`;
				context.report({
					node: prop.node,
					messageId: `missingDefault`,
					data: { propName }
				});
			} else if (prop.type === "type") {
				if (prop.required || prop.types.length === 1 && prop.types[0] === "Boolean" || shouldIgnore?.(prop)) continue;
				context.report({
					node: prop.node,
					messageId: `missingDefault`,
					data: { propName: prop.propName }
				});
			}
		}
		return require_index.default.compositingVisitors(require_index.default.defineScriptSetupVisitor(context, { onDefinePropsEnter(node, props) {
			const hasWithDefaults = require_index.default.hasWithDefaults(node);
			const defaultsByWithDefaults = require_index.default.getWithDefaultsPropExpressions(context, node);
			const isUsingPropsDestructure = require_index.default.isUsingPropsDestructure(node);
			const defaultsByAssignmentPatterns = require_index.default.getDefaultPropExpressionsForPropsDestructure(node);
			processProps(props, (prop) => prop.type === "type" && (!hasWithDefaults && !isUsingPropsDestructure || defaultsByWithDefaults.hasUnknown || Boolean(defaultsByWithDefaults.props[prop.propName])) || isUsingPropsDestructure && (prop.propName == null || Boolean(defaultsByAssignmentPatterns[prop.propName])));
		} }), require_index.default.executeOnVue(context, (obj) => {
			processProps(require_index.default.getComponentPropsFromOptions(obj));
		}));
	}
};
exports.default = require_default_prop_default;
