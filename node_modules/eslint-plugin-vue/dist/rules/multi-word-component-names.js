const require_runtime = require("../_virtual/_rolldown/runtime.js");
const require_casing = require("../utils/casing.js");
const require_index = require("../utils/index.js");
let node_path = require("node:path");
node_path = require_runtime.__toESM(node_path);
// lib/rules/multi-word-component-names.ts
/**
* @author Marton Csordas
* See LICENSE file in root directory for full license.
*/
var multi_word_component_names_default = {
	meta: {
		type: "suggestion",
		docs: {
			description: "require component names to be always multi-word",
			categories: ["vue3-essential", "vue2-essential"],
			url: "https://eslint.vuejs.org/rules/multi-word-component-names.html"
		},
		schema: [{
			type: "object",
			properties: { ignores: {
				type: "array",
				items: { type: "string" },
				uniqueItems: true
			} },
			additionalProperties: false
		}],
		messages: { unexpected: "Component name \"{{value}}\" should always be multi-word." }
	},
	create(context) {
		const ignores = /* @__PURE__ */ new Set(["App", "app"]);
		const optionIgnores = context.options[0] && context.options[0].ignores || [];
		for (const ignore of optionIgnores) {
			ignores.add(ignore);
			if (require_casing.isPascalCase(ignore)) ignores.add(require_casing.kebabCase(ignore));
		}
		let hasVue = require_index.default.isScriptSetup(context);
		let hasName = false;
		/**
		* Returns true if the given component name is valid, otherwise false.
		* */
		function isValidComponentName(name) {
			if (ignores.has(name) || require_index.default.VUE3_BUILTIN_COMPONENT_NAMES.has(name)) return true;
			return require_casing.kebabCase(name).split("-").length > 1;
		}
		function validateName(nameNode) {
			if (nameNode.type !== "Literal") return;
			const componentName = String(nameNode.value);
			if (!isValidComponentName(componentName)) context.report({
				node: nameNode,
				messageId: "unexpected",
				data: { value: componentName }
			});
		}
		return require_index.default.compositingVisitors(require_index.default.executeOnCallVueComponent(context, (node) => {
			hasVue = true;
			if (node.arguments.length !== 2) return;
			hasName = true;
			validateName(node.arguments[0]);
		}), require_index.default.executeOnVue(context, (obj) => {
			hasVue = true;
			const node = require_index.default.findProperty(obj, "name");
			if (!node) return;
			hasName = true;
			validateName(node.value);
		}), require_index.default.defineScriptSetupVisitor(context, { onDefineOptionsEnter(node) {
			if (node.arguments.length === 0) return;
			const define = node.arguments[0];
			if (define.type !== "ObjectExpression") return;
			const nameNode = require_index.default.findProperty(define, "name");
			if (!nameNode) return;
			hasName = true;
			validateName(nameNode.value);
		} }), { "Program:exit"(node) {
			if (hasName || !hasVue && node.body.length > 0) return;
			const fileName = context.filename;
			const componentName = node_path.default.basename(fileName, node_path.default.extname(fileName));
			if (require_index.default.isVueFile(fileName) && !isValidComponentName(componentName)) context.report({
				messageId: "unexpected",
				data: { value: componentName },
				loc: {
					line: 1,
					column: 0
				}
			});
		} });
	}
};
exports.default = multi_word_component_names_default;
