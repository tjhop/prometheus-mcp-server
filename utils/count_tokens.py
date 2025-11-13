import sys
import tiktoken

"""Usage: uv run --with tiktoken count_tokens.py <filepath> """

def count_tokens(text: str) -> int:
    """Calculates the number of tokens in a given text input using tiktoken and gpt-5 encoding."""
    enc = tiktoken.encoding_for_model("gpt-5")
    encoded = enc.encode(text)
    return len(encoded)

def read_file(filepath: str) -> str:
    """Reads the content of a file and returns it as a string."""
    with open(filepath, 'r', encoding='utf-8') as f:
        return f.read()

def main():
    """Main entry point for the token counting script."""
    if len(sys.argv) < 2:
        print(f"Usage: python3 {sys.argv[1]} <filepath>")
        sys.exit(1)

    filepath = sys.argv[1]
    try:
        file_content = read_file(filepath)
        num_tokens = count_tokens(file_content)
        print(f"File: {filepath}")
        print(f"Number of tokens: {num_tokens}")
    except FileNotFoundError:
        print(f"Error: File not found at '{filepath}'")
        sys.exit(1)
    except Exception as e:
        print(f"An error occurred: {e}")
        sys.exit(1)

if __name__ == "__main__":
    main()

