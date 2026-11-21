const require_index = require("../../utils/index.js");
// lib/rules/syntaxes/define-options.ts
/**
* @author Yosuke Ota
* See LICENSE file in root directory for full license.
*/
var define_options_default = {
	supported: ">=3.3.0",
	createScriptVisitor(context) {
		const sourceCode = context.sourceCode;
		return require_index.default.defineScriptSetupVisitor(context, { onDefineOptionsEnter(node) {
			context.report({
				node,
				messageId: "forbiddenDefineOptions",
				fix(fixer) {
					return fix(fixer, node);
				}
			});
		} });
		/**
		* Moves the options of the given `defineOptions()` call to `<script>`.
		*/
		function fix(fixer, node) {
			if (node.arguments.length === 0) return null;
			const scriptSetup = require_index.default.getScriptSetupElement(context);
			if (!scriptSetup || scriptSetup.parent.children.filter(require_index.default.isVElement).some((node) => node.name === "script" && !require_index.default.hasAttribute(node, "setup"))) return null;
			let statement = node;
			while (statement.parent && statement.parent.type !== "Program") statement = statement.parent;
			const removeRange = [...statement.range];
			if (sourceCode.lines[statement.loc.start.line - 1].slice(0, statement.loc.start.column).trim() === "") removeRange[0] -= statement.loc.start.column;
			return [fixer.insertTextBefore(scriptSetup, `<script>\nexport default ${sourceCode.getText(node.arguments[0])}\n<\/script>\n`), fixer.removeRange(removeRange)];
		}
	}
};
exports.default = define_options_default;
