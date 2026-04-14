import importlib.util
import sys
import types
import unittest
from pathlib import Path


SCRIPT_PATH = Path(__file__).with_name("gepa_segmentation.py")


def load_module():
    dspy = types.ModuleType("dspy")

    class Signature:
        pass

    class Module:
        pass

    class Prediction:
        def __init__(self, **kwargs):
            self.__dict__.update(kwargs)

    class Example:
        def __init__(self, **kwargs):
            self.__dict__.update(kwargs)

        def with_inputs(self, *_args):
            return self

    class Predict:
        def __init__(self, *_args, **_kwargs):
            pass

        def __call__(self, **kwargs):
            return Prediction(**kwargs)

    class LM:
        def __init__(self, *_args, **_kwargs):
            pass

    class GEPA:
        def __init__(self, *_args, **_kwargs):
            pass

    dspy.Signature = Signature
    dspy.Module = Module
    dspy.Prediction = Prediction
    dspy.Example = Example
    dspy.Predict = Predict
    dspy.LM = LM
    dspy.GEPA = GEPA
    dspy.InputField = lambda **_kwargs: None
    dspy.OutputField = lambda **_kwargs: None
    dspy.configure = lambda **_kwargs: None

    dotenv = types.ModuleType("dotenv")
    dotenv.load_dotenv = lambda *args, **kwargs: None

    sys.modules["dspy"] = dspy
    sys.modules["dotenv"] = dotenv
    sys.modules.pop("gepa_segmentation_under_test", None)

    spec = importlib.util.spec_from_file_location("gepa_segmentation_under_test", SCRIPT_PATH)
    if spec is None or spec.loader is None:
        raise RuntimeError("failed to load gepa_segmentation module spec")

    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


class GepaPathLayoutTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.module = load_module()

    def test_resolve_repo_path_anchors_relative_paths_at_repo_root(self):
        got = self.module.resolve_repo_path("data/jepa/datasets/paragraphs.csv")
        want = self.module.REPO_ROOT / "data" / "jepa" / "datasets" / "paragraphs.csv"
        self.assertEqual(got, want)

    def test_build_artifact_paths_keeps_compiled_instruction_at_root(self):
        paths = self.module.build_artifact_paths(Path("/tmp/gepa-output"))

        self.assertEqual(
            paths["compiled_instruction"],
            Path("/tmp/gepa-output/compiled_instruction.txt"),
        )
        self.assertEqual(paths["runs_dir"], Path("/tmp/gepa-output/runs"))
        self.assertEqual(
            paths["compile_metadata"],
            Path("/tmp/gepa-output/runs/compile_metadata.json"),
        )


if __name__ == "__main__":
    unittest.main()
