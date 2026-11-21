const require_index = require("../utils/index.js");
// lib/rules/require-default-export.ts
/**
* @author ItMaga
* See LICENSE file in root directory for full license.
*/
var require_default_export_default = {
	meta: {
		type: "problem",
		docs: {
			description: "require components to be the default export",
			categories: void 0,
			url: "https://eslint.vuejs.org/rules/require-default-export.html"
		},
		fixable: null,
		schema: [],
		messages: {
			missing: "Missing default export.",
			mustBeDefaultExport: "Component must be the default export."
		}
	},
	create(context) {
		const documentFragment = context.sourceCode.parserServices.getDocumentFragment?.();
		if (!(documentFragment && documentFragment.children.some((e) => require_index.default.isVElement(e) && e.name === "script")) || require_index.default.isScriptSetup(context)) return {};
		let hasDefaultExport = false;
		let hasDefinedComponent = false;
		return require_index.default.compositingVisitors(require_index.default.defineVueVisitor(context, { onVueObjectExit() {
			hasDefinedComponent = true;
		} }), {
			"Program > ExportDefaultDeclaration"() {
				hasDefaultExport = true;
			},
			"Program:exit"(node) {
				if (!hasDefaultExport && node.body.length > 0) context.report({
					loc: node.tokens.at(-1).loc,
					messageId: hasDefinedComponent ? "mustBeDefaultExport" : "missing"
				});
			}
		});
	}
};
exports.default = require_default_export_default;
