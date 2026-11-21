const require_index = require("../utils/index.js");
// lib/rules/prefer-define-options.ts
/**
* @author Yosuke Ota <https://github.com/ota-meshi>
* See LICENSE file in root directory for full license.
*/
var prefer_define_options_default = {
	meta: {
		type: "suggestion",
		docs: {
			description: "enforce use of `defineOptions` instead of default export",
			categories: void 0,
			url: "https://eslint.vuejs.org/rules/prefer-define-options.html"
		},
		fixable: "code",
		schema: [],
		messages: { preferDefineOptions: "Use `defineOptions` instead of default export." }
	},
	create(context) {
		const scriptSetup = require_index.default.getScriptSetupElement(context);
		if (!scriptSetup) return {};
		let defineOptionsNode = null;
		let exportDefaultDeclaration = null;
		let lastImportDeclaration = null;
		return require_index.default.compositingVisitors(require_index.default.defineScriptSetupVisitor(context, {
			ImportDeclaration(node) {
				lastImportDeclaration = node;
			},
			onDefineOptionsEnter(node) {
				defineOptionsNode = node;
			}
		}), {
			ExportDefaultDeclaration(node) {
				exportDefaultDeclaration = node;
			},
			"Program:exit"() {
				if (!exportDefaultDeclaration) return;
				context.report({
					node: exportDefaultDeclaration,
					messageId: "preferDefineOptions",
					fix: defineOptionsNode ? null : buildFix(exportDefaultDeclaration, scriptSetup)
				});
			}
		});
		function buildFix(node, scriptSetup) {
			return (fixer) => {
				const sourceCode = context.sourceCode;
				let removeRange = [...node.range];
				const script = scriptSetup.parent.children.filter(require_index.default.isVElement).find((node) => node.name === "script" && !require_index.default.hasAttribute(node, "setup"));
				if (script && script.endTag && sourceCode.getTokensBetween(script.startTag, script.endTag, { includeComments: true }).every((token) => removeRange[0] <= token.range[0] && token.range[1] <= removeRange[1])) removeRange = [...script.range];
				const removeStartLoc = sourceCode.getLocFromIndex(removeRange[0]);
				if (sourceCode.lines[removeStartLoc.line - 1].slice(0, removeStartLoc.column).trim() === "") removeRange[0] = removeStartLoc.line === 1 ? 0 : sourceCode.getIndexFromLoc({
					line: removeStartLoc.line - 1,
					column: sourceCode.lines[removeStartLoc.line - 2].length
				});
				const insertAfterTag = lastImportDeclaration || scriptSetup.startTag;
				return [fixer.removeRange(removeRange), fixer.insertTextAfter(insertAfterTag, `\ndefineOptions(${sourceCode.getText(node.declaration)})\n`)];
			};
		}
	}
};
exports.default = prefer_define_options_default;
