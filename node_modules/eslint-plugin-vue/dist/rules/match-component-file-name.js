const require_runtime = require("../_virtual/_rolldown/runtime.js");
const require_index = require("../utils/index.js");
const require_casing = require("../utils/casing.js");
let node_path = require("node:path");
node_path = require_runtime.__toESM(node_path);
// lib/rules/match-component-file-name.ts
/**
* @fileoverview Require component name property to match its file name
* @author Rodrigo Pedra Brum <rodrigo.pedra@gmail.com>
*/
var match_component_file_name_default = {
	meta: {
		type: "suggestion",
		docs: {
			description: "require component name property to match its file name",
			categories: void 0,
			url: "https://eslint.vuejs.org/rules/match-component-file-name.html"
		},
		fixable: null,
		hasSuggestions: true,
		schema: [{
			type: "object",
			properties: {
				extensions: {
					type: "array",
					items: { type: "string" },
					uniqueItems: true
				},
				shouldMatchCase: { type: "boolean" }
			},
			additionalProperties: false
		}],
		messages: { shouldMatchFileName: "Component name `{{name}}` should match file name `{{filename}}`." }
	},
	create(context) {
		const options = context.options[0];
		const shouldMatchCase = options && options.shouldMatchCase || false;
		const extensionsArray = options && options.extensions;
		const allowedExtensions = Array.isArray(extensionsArray) ? extensionsArray : ["jsx"];
		const extension = node_path.default.extname(context.filename);
		if (!allowedExtensions.includes(extension.replace(/^\./, ""))) return {};
		const filename = node_path.default.basename(context.filename, extension);
		const errors = [];
		let componentCount = 0;
		function areNamesEquivalent(name, filename) {
			return shouldMatchCase ? name === filename : require_casing.pascalCase(name) === filename || require_casing.kebabCase(name) === filename;
		}
		function verifyName(node) {
			const name = require_index.default.getStringLiteralValue(node);
			if (name === null || areNamesEquivalent(name, filename)) return;
			errors.push({
				node,
				messageId: "shouldMatchFileName",
				data: {
					filename,
					name
				},
				suggest: [{
					desc: "Rename component to match file name.",
					fix(fixer) {
						const quote = node.type === "TemplateLiteral" ? "`" : node.raw[0];
						return fixer.replaceText(node, `${quote}${filename}${quote}`);
					}
				}]
			});
		}
		return require_index.default.compositingVisitors(require_index.default.executeOnCallVueComponent(context, (node) => {
			if (node.arguments.length !== 2) return;
			const argument = node.arguments[0];
			if (require_index.default.isStringLiteral(argument)) verifyName(argument);
		}), require_index.default.executeOnVue(context, (object) => {
			const node = require_index.default.findProperty(object, "name");
			componentCount++;
			if (!node || !require_index.default.isStringLiteral(node.value)) return;
			verifyName(node.value);
		}), require_index.default.defineScriptSetupVisitor(context, { onDefineOptionsEnter(node) {
			componentCount++;
			if (node.arguments.length === 0) return;
			const define = node.arguments[0];
			if (define.type !== "ObjectExpression") return;
			const nameNode = require_index.default.findProperty(define, "name");
			if (!nameNode || !require_index.default.isStringLiteral(nameNode.value)) return;
			verifyName(nameNode.value);
		} }), { "Program:exit"() {
			if (componentCount > 1) return;
			for (const error of errors) context.report(error);
		} });
	}
};
exports.default = match_component_file_name_default;
