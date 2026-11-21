const require_index = require("../utils/index.js");
// lib/rules/no-boolean-default.ts
/**
* @fileoverview Prevents boolean defaults from being set
* @author Hiroki Osame
*/
function isBooleanIdentifier(node) {
	return Boolean(node && node.type === "Identifier" && node.name === "Boolean");
}
/**
* Detects whether given prop node is a Boolean
*/
function isBooleanProp(prop) {
	const value = require_index.default.skipTSAsExpression(prop.value);
	return isBooleanIdentifier(value) || value.type === "ObjectExpression" && isBooleanIdentifier(require_index.default.findProperty(value, "type")?.value);
}
function getDefaultNode(propDefValue) {
	return require_index.default.findProperty(propDefValue, "default");
}
var no_boolean_default_default = {
	meta: {
		type: "suggestion",
		docs: {
			description: "disallow boolean defaults",
			categories: void 0,
			url: "https://eslint.vuejs.org/rules/no-boolean-default.html"
		},
		fixable: null,
		schema: [{ enum: ["default-false", "no-default"] }],
		messages: {
			noBooleanDefault: "Boolean prop should not set a default (Vue defaults it to false).",
			defaultFalse: "Boolean prop should only be defaulted to false."
		}
	},
	create(context) {
		const booleanType = context.options[0] || "no-default";
		function processProp(prop, otherDefaultProvider) {
			if (prop.type === "object") {
				if (!isBooleanProp(prop)) return;
				if (prop.value.type === "ObjectExpression") {
					const defaultNode = getDefaultNode(prop.value);
					if (defaultNode) verifyDefaultExpression(defaultNode.value);
				}
				if (prop.propName != null) for (const defaultNode of otherDefaultProvider(prop.propName)) verifyDefaultExpression(defaultNode);
			} else if (prop.type === "type") {
				if (prop.types.length !== 1 || prop.types[0] !== "Boolean") return;
				for (const defaultNode of otherDefaultProvider(prop.propName)) verifyDefaultExpression(defaultNode);
			}
		}
		function processProps(props, otherDefaultProvider) {
			for (const prop of props) processProp(prop, otherDefaultProvider);
		}
		function verifyDefaultExpression(defaultNode) {
			switch (booleanType) {
				case "no-default":
					context.report({
						node: defaultNode,
						messageId: "noBooleanDefault"
					});
					break;
				case "default-false": if (defaultNode.type !== "Literal" || defaultNode.value !== false) context.report({
					node: defaultNode,
					messageId: "defaultFalse"
				});
			}
		}
		return require_index.default.compositingVisitors(require_index.default.executeOnVueComponent(context, (obj) => {
			processProps(require_index.default.getComponentPropsFromOptions(obj), () => []);
		}), require_index.default.defineScriptSetupVisitor(context, { onDefinePropsEnter(node, props) {
			const { props: defaultsByWithDefaults } = require_index.default.getWithDefaultsPropExpressions(context, node);
			const defaultsByAssignmentPatterns = require_index.default.getDefaultPropExpressionsForPropsDestructure(node);
			processProps(props, (propName) => [defaultsByWithDefaults[propName], defaultsByAssignmentPatterns[propName]?.expression].filter(require_index.default.isDef));
		} }));
	}
};
exports.default = no_boolean_default_default;
