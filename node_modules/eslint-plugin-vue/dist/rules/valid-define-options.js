const require_index = require("../utils/index.js");
let _eslint_community_eslint_utils = require("@eslint-community/eslint-utils");
// lib/rules/valid-define-options.ts
/**
* @author Yosuke Ota <https://github.com/ota-meshi>
* See LICENSE file in root directory for full license.
*/
var valid_define_options_default = {
	meta: {
		type: "problem",
		docs: {
			description: "enforce valid `defineOptions` compiler macro",
			categories: ["vue3-essential"],
			url: "https://eslint.vuejs.org/rules/valid-define-options.html"
		},
		fixable: null,
		schema: [],
		messages: {
			referencingLocally: "`defineOptions` is referencing locally declared variables.",
			multiple: "`defineOptions` has been called multiple times.",
			notDefined: "Options are not defined.",
			disallowProp: "`defineOptions()` cannot be used to declare `{{propName}}`. Use `{{insteadMacro}}()` instead.",
			typeArgs: "`defineOptions()` cannot accept type arguments."
		}
	},
	create(context) {
		const scriptSetup = require_index.default.getScriptSetupElement(context);
		if (!scriptSetup) return {};
		const optionsDefExpressions = /* @__PURE__ */ new Set();
		const defineOptionsNodes = [];
		return require_index.default.compositingVisitors(require_index.default.defineScriptSetupVisitor(context, {
			onDefineOptionsEnter(node) {
				defineOptionsNodes.push(node);
				if (node.arguments.length > 0) {
					const define = node.arguments[0];
					if (define.type === "ObjectExpression") {
						for (const [propName, insteadMacro] of [
							["props", "defineProps"],
							["emits", "defineEmits"],
							["expose", "defineExpose"],
							["slots", "defineSlots"]
						]) if (require_index.default.findProperty(define, propName)) context.report({
							node,
							messageId: "disallowProp",
							data: {
								propName,
								insteadMacro
							}
						});
					}
					optionsDefExpressions.add(node.arguments[0]);
				} else context.report({
					node,
					messageId: "notDefined"
				});
				const typeArguments = "typeArguments" in node ? node.typeArguments : node.typeParameters;
				if (typeArguments) context.report({
					node: typeArguments,
					messageId: "typeArgs"
				});
			},
			Identifier(node) {
				for (const defineOptions of optionsDefExpressions) {
					if (!require_index.default.isInRange(defineOptions.range, node)) continue;
					const variable = (0, _eslint_community_eslint_utils.findVariable)(require_index.default.getScope(context, node), node);
					if (!variable || variable.references.every((ref) => ref.identifier !== node) || variable.defs.length === 0 || variable.defs.some((def) => def.type === "ImportBinding" || !require_index.default.isInRange(scriptSetup.range, def.name) || require_index.default.isInRange(defineOptions.range, def.name) || def.type === "Variable" && def.parent.kind === "const" && def.node.init && def.node.init.type === "Literal") || require_index.default.isWithinTypeNode(node)) continue;
					context.report({
						node,
						messageId: "referencingLocally"
					});
				}
			}
		}), { "Program:exit"() {
			if (defineOptionsNodes.length > 1) for (const node of defineOptionsNodes) context.report({
				node,
				messageId: "multiple"
			});
		} });
	}
};
exports.default = valid_define_options_default;
