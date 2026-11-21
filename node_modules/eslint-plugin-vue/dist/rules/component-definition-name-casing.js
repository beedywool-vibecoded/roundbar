const require_index = require("../utils/index.js");
const require_casing = require("../utils/casing.js");
// lib/rules/component-definition-name-casing.ts
/**
* @fileoverview enforce specific casing for component definition name
* @author Armano
*/
const allowedCaseOptions = ["PascalCase", "kebab-case"];
var component_definition_name_casing_default = {
	meta: {
		type: "suggestion",
		docs: {
			description: "enforce specific casing for component definition name",
			categories: ["vue3-strongly-recommended", "vue2-strongly-recommended"],
			url: "https://eslint.vuejs.org/rules/component-definition-name-casing.html"
		},
		fixable: "code",
		schema: [{ enum: allowedCaseOptions }],
		messages: { incorrectCase: "Property name \"{{value}}\" is not {{caseType}}." }
	},
	create(context) {
		const options = context.options[0];
		const caseType = allowedCaseOptions.includes(options) ? options : "PascalCase";
		function convertName(node) {
			const nodeValue = require_index.default.getStringLiteralValue(node);
			if (nodeValue === null || require_casing.getChecker(caseType)(nodeValue)) return;
			context.report({
				node,
				messageId: "incorrectCase",
				data: {
					value: nodeValue,
					caseType
				},
				fix: (fixer) => fixer.replaceTextRange([node.range[0] + 1, node.range[1] - 1], require_casing.getExactConverter(caseType)(nodeValue))
			});
		}
		return require_index.default.compositingVisitors(require_index.default.executeOnCallVueComponent(context, (node) => {
			if (node.arguments.length !== 2) return;
			const argument = node.arguments[0];
			if (require_index.default.isStringLiteral(argument)) convertName(argument);
		}), require_index.default.executeOnVue(context, (obj) => {
			const node = require_index.default.findProperty(obj, "name");
			if (!node || !require_index.default.isStringLiteral(node.value)) return;
			convertName(node.value);
		}), require_index.default.defineScriptSetupVisitor(context, { onDefineOptionsEnter(node) {
			if (node.arguments.length === 0) return;
			const define = node.arguments[0];
			if (define.type !== "ObjectExpression") return;
			const nameNode = require_index.default.findProperty(define, "name");
			if (!nameNode || !require_index.default.isStringLiteral(nameNode.value)) return;
			convertName(nameNode.value);
		} }));
	}
};
exports.default = component_definition_name_casing_default;
